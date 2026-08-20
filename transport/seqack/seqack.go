// Package seqack implements api:sequence-ack-layer: the transport
// frontend that numbers outgoing datagrams, reports which ones arrived,
// and surfaces what the peer has confirmed. Both ends run the same layer.
//
// What it feeds upward: the newest confirmed application tag (the state
// tick a statesync sender uses as its confirmed baseline under
// concept:delta-baseline-policy), an RTT sample, a loss estimate, and the
// last-receive instant for silence detection (the deadline itself is
// measured in missed ticks by the caller, per data:session-tuning-profile).
//
// Datagram layout (big-endian): 0xEA, version(1), flags, seq uint32,
// ackSeq uint32, ackBits uint32, payload. flags bit0 set marks a
// dedicated ack with no payload (concept:ack-transmission-policy).
//
// Stale datagrams — sequence at or below the newest delivered — are
// acked but not delivered: the state stream is last-writer-wins, so an
// out-of-order arrival carries nothing the receiver still wants.
package seqack

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"github.com/shibukawa/ebigentserver/transport"
)

const (
	marker  = 0xEA
	version = 1
	header  = 15
	flagAck = 1
)

// Policy is concept:ack-transmission-policy.
type Policy uint8

const (
	// PiggybackOnly attaches the ack record to outgoing datagrams and
	// never emits its own; it requires a return flow at a comparable
	// rate.
	PiggybackOnly Policy = iota
	// Dedicated emits standalone acks for every advance — the mode for
	// receive-only participants such as spectators, whose baseline
	// would otherwise never confirm.
	Dedicated
	// DelayedPiggyback waits AckDeadline for an outgoing datagram to
	// carry the ack, then degrades to a dedicated one.
	DelayedPiggyback
)

// Stats is the measured link state consumed by
// concept:delta-baseline-policy's adaptive mode.
type Stats struct {
	// RTT is the newest round-trip sample; zero until one exists.
	RTT time.Duration
	// LossPercent estimates datagram loss over the recent window.
	LossPercent int32
	// LastReceived is when anything last arrived, for silence
	// detection.
	LastReceived time.Time
}

// Layer runs over one Conn's unreliable channel. Safe for concurrent use.
type Layer struct {
	conn        transport.Conn
	policy      Policy
	ackDeadline time.Duration
	now         func() time.Time

	mu sync.Mutex
	// send state
	nextSeq  uint32
	inflight []flight // ring of recent sends, oldest first
	// receive state
	recvHighest   uint32 // highest sequence seen
	recvBits      uint32 // receipts of recvHighest-1 .. -32
	delivered     uint32 // highest sequence handed to the caller
	deliveredAny  bool
	anyReceived   bool
	lastReceived  time.Time
	ackDirty      bool      // receipts not yet reported to the peer
	ackDirtySince time.Time // when ackDirty became true
	// peer-confirmed state
	confirmedTag uint64
	confirmedOK  bool
	// measurements
	rtt      time.Duration
	ackedN   int32 // over the retired window
	expiredN int32
}

type flight struct {
	seq    uint32
	tag    uint64
	sentAt time.Time
	acked  bool
}

// inflightWindow bounds the send bookkeeping; beyond it the oldest entry
// retires into the loss estimate.
const inflightWindow = 128

// lossAge retires an unacked send into the loss estimate by age, so slow
// senders still measure loss.
const lossAge = time.Second

// Options tunes the layer.
type Options struct {
	// Policy selects the ack transmission mode.
	Policy Policy
	// AckDeadline bounds how long DelayedPiggyback waits; required for
	// that policy.
	AckDeadline time.Duration
	// Now overrides the clock (tests).
	Now func() time.Time
}

// New wraps a connection.
func New(conn transport.Conn, opts Options) *Layer {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Layer{conn: conn, policy: opts.Policy, ackDeadline: opts.AckDeadline, now: opts.Now}
}

// SendDatagram numbers and sends one payload, piggybacking the current
// ack record. tag is the application version this datagram carries (the
// state tick); its confirmation surfaces through Confirmed.
func (l *Layer) SendDatagram(ctx context.Context, payload []byte, tag uint64) error {
	l.mu.Lock()
	now := l.now()
	seq := l.nextSeq
	l.nextSeq++
	l.inflight = append(l.inflight, flight{seq: seq, tag: tag, sentAt: now})
	for len(l.inflight) > 0 &&
		(len(l.inflight) > inflightWindow || now.Sub(l.inflight[0].sentAt) > lossAge) {
		l.retire(l.inflight[0])
		l.inflight = l.inflight[1:]
	}
	frame := l.frameLocked(0, seq, payload)
	l.ackDirty = false
	l.mu.Unlock()
	return l.conn.SendUnreliable(ctx, frame)
}

// MaybeFlushAck emits a standalone ack when the policy calls for one:
// always under Dedicated (if receipts are unreported), after the deadline
// under DelayedPiggyback. Drive it from the receive loop or a ticker.
func (l *Layer) MaybeFlushAck(ctx context.Context) error {
	l.mu.Lock()
	due := l.ackDirty && (l.policy == Dedicated ||
		(l.policy == DelayedPiggyback && l.now().Sub(l.ackDirtySince) >= l.ackDeadline))
	if !due {
		l.mu.Unlock()
		return nil
	}
	seq := l.nextSeq
	l.nextSeq++
	frame := l.frameLocked(flagAck, seq, nil)
	l.ackDirty = false
	l.mu.Unlock()
	return l.conn.SendUnreliable(ctx, frame)
}

// frameLocked builds a datagram carrying the current ack record.
func (l *Layer) frameLocked(flags byte, seq uint32, payload []byte) []byte {
	frame := make([]byte, header, header+len(payload))
	frame[0], frame[1], frame[2] = marker, version, flags
	binary.BigEndian.PutUint32(frame[3:7], seq)
	binary.BigEndian.PutUint32(frame[7:11], l.recvHighest)
	binary.BigEndian.PutUint32(frame[11:15], l.recvBits)
	return append(frame, payload...)
}

// Absorb consumes one received unreliable payload. It returns the
// application payload when the datagram carries a new one, or nil for
// dedicated acks, stale arrivals, and frames that are not ours.
func (l *Layer) Absorb(frame []byte) []byte {
	if len(frame) < header || frame[0] != marker || frame[1] != version {
		return nil
	}
	flags := frame[2]
	seq := binary.BigEndian.Uint32(frame[3:7])
	ackSeq := binary.BigEndian.Uint32(frame[7:11])
	ackBits := binary.BigEndian.Uint32(frame[11:15])

	l.mu.Lock()
	now := l.now()
	l.lastReceived = now
	l.noteReceiptLocked(seq)
	l.processAckLocked(ackSeq, ackBits, now)
	deliver := flags&flagAck == 0 && (!l.deliveredAny || newer(seq, l.delivered))
	if deliver {
		l.delivered = seq
		l.deliveredAny = true
	}
	l.mu.Unlock()

	if !deliver {
		return nil
	}
	return frame[header:]
}

// noteReceiptLocked folds a received sequence into the highest+bitfield
// record.
func (l *Layer) noteReceiptLocked(seq uint32) {
	l.ackDirty = true
	l.ackDirtySince = l.lastReceived
	if !l.anyReceived {
		l.anyReceived = true
		l.recvHighest = seq
		return
	}
	if newer(seq, l.recvHighest) {
		shift := seq - l.recvHighest
		if shift >= 32 {
			l.recvBits = 0
		} else {
			l.recvBits = l.recvBits<<shift | 1<<(shift-1)
		}
		l.recvHighest = seq
		return
	}
	back := l.recvHighest - seq
	if back >= 1 && back <= 32 {
		l.recvBits |= 1 << (back - 1)
	}
}

// processAckLocked marks in-flight sends the peer reports holding.
func (l *Layer) processAckLocked(ackSeq, ackBits uint32, now time.Time) {
	for i := range l.inflight {
		f := &l.inflight[i]
		if f.acked {
			continue
		}
		var got bool
		if f.seq == ackSeq {
			got = true
		} else if newer(ackSeq, f.seq) {
			back := ackSeq - f.seq
			got = back <= 32 && ackBits&(1<<(back-1)) != 0
		}
		if got {
			f.acked = true
			l.rtt = now.Sub(f.sentAt)
			if !l.confirmedOK || f.tag > l.confirmedTag {
				l.confirmedTag, l.confirmedOK = f.tag, true
			}
			l.ackedN++
		}
	}
}

// retire folds a send falling out of the window into the loss estimate.
func (l *Layer) retire(f flight) {
	if f.acked {
		return
	}
	l.expiredN++
}

// Confirmed reports the newest application tag the peer is known to hold
// — the confirmed baseline of concept:delta-baseline-policy.
func (l *Layer) Confirmed() (uint64, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.confirmedTag, l.confirmedOK
}

// Stats reports the measured link state.
func (l *Layer) Stats() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := Stats{RTT: l.rtt, LastReceived: l.lastReceived}
	if total := l.ackedN + l.expiredN; total > 0 {
		s.LossPercent = l.expiredN * 100 / total
	}
	return s
}

// newer reports whether a follows b in sequence space, wraparound-safe.
func newer(a, b uint32) bool { return int32(a-b) > 0 }
