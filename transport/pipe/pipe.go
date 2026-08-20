// Package pipe is the in-process transport pair, with fault injection.
// Tests and local topologies use it as-is; with a Faults declaration it
// becomes the loss-and-latency rig Phase 3's completion criterion asks
// for ("Pong survives injected loss and delay").
//
// Determinism note: injected faults use a seeded fixmath PRNG, so a run's
// drop/reorder pattern is reproducible; deliveries ride on real timers,
// so arrival timing is not — which is the point of the rig.
package pipe

import (
	"context"
	"sync"
	"time"

	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/fixmath"
)

// Faults declares the injected link behavior for one direction. The
// reliable channel honors Latency/Jitter only (it may be slow, never
// lossy); the unreliable channel honors everything.
type Faults struct {
	// LossPercent drops that percentage of unreliable datagrams.
	LossPercent int32
	// Latency delays every delivery.
	Latency time.Duration
	// Jitter adds up to this much extra, uniformly, per message.
	Jitter time.Duration
	// ReorderPercent holds that percentage of unreliable datagrams back
	// until the next datagram passes them.
	ReorderPercent int32
	// Seed drives the fault PRNG.
	Seed uint64
}

type timed struct {
	at  time.Time
	msg transport.Message
}

// Pair returns two connected endpoints. faultsAB shapes messages from a
// to b; faultsBA the reverse. Zero-value Faults is a perfect link.
func Pair(faultsAB, faultsBA Faults) (a, b transport.Conn) {
	ea := newEnd(faultsAB)
	eb := newEnd(faultsBA)
	ea.peer, eb.peer = eb, ea
	go ea.pumpReliable()
	go eb.pumpReliable()
	return ea, eb
}

type end struct {
	peer   *end
	inbox  chan transport.Message
	relQ   chan timed // ordered reliable deliveries toward peer
	done   chan struct{}
	closed sync.Once

	mu      sync.Mutex // guards rng and pending
	f       Faults
	rng     fixmath.Rand
	pending *transport.Message
}

func newEnd(f Faults) *end {
	return &end{
		inbox: make(chan transport.Message, 256),
		relQ:  make(chan timed, 256),
		done:  make(chan struct{}),
		f:     f,
		rng:   fixmath.NewRand(f.Seed | 1),
	}
}

var _ transport.Conn = (*end)(nil)

func (e *end) Capability() transport.Capability {
	return transport.Capability{ReliableStream: true, UnreliableDatagram: true}
}

// pumpReliable delivers queued reliable messages in order, honoring each
// message's delivery time.
func (e *end) pumpReliable() {
	for {
		select {
		case <-e.done:
			return
		case t := <-e.relQ:
			if d := time.Until(t.at); d > 0 {
				select {
				case <-time.After(d):
				case <-e.done:
					return
				}
			}
			select {
			case e.peer.inbox <- t.msg:
			case <-e.done:
				return
			case <-e.peer.done:
				return
			}
		}
	}
}

func (e *end) SendReliable(ctx context.Context, payload []byte) error {
	e.mu.Lock()
	d := e.delayLocked()
	e.mu.Unlock()
	t := timed{at: time.Now().Add(d), msg: transport.Message{Channel: transport.Reliable, Payload: clone(payload)}}
	select {
	case e.relQ <- t:
		return nil
	case <-e.done:
		return transport.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	default:
		return transport.ErrBackpressure
	}
}

func (e *end) SendUnreliable(ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-e.done:
		return transport.ErrClosed
	default:
	}
	msg := transport.Message{Channel: transport.Unreliable, Payload: clone(payload)}
	e.mu.Lock()
	if e.f.LossPercent > 0 && e.rollLocked() < e.f.LossPercent {
		e.mu.Unlock()
		return nil // dropped, as datagrams may be
	}
	var release *transport.Message
	if e.pending != nil {
		release, e.pending = e.pending, nil
	} else if e.f.ReorderPercent > 0 && e.rollLocked() < e.f.ReorderPercent {
		held := msg
		e.pending = &held
		e.mu.Unlock()
		return nil // delivered when the next datagram passes it
	}
	d := e.delayLocked()
	e.mu.Unlock()

	e.deliverDatagram(msg, d)
	if release != nil {
		e.deliverDatagram(*release, d)
	}
	return nil
}

// deliverDatagram hands one datagram to the peer after its delay; a full
// peer inbox drops it, as datagrams may be.
func (e *end) deliverDatagram(msg transport.Message, d time.Duration) {
	send := func() {
		select {
		case e.peer.inbox <- msg:
		default:
		}
	}
	if d <= 0 {
		send()
		return
	}
	time.AfterFunc(d, func() {
		select {
		case <-e.peer.done:
		default:
			send()
		}
	})
}

func (e *end) Receive(ctx context.Context) (transport.Message, error) {
	select {
	case msg := <-e.inbox:
		return msg, nil
	default:
	}
	select {
	case msg := <-e.inbox:
		return msg, nil
	case <-ctx.Done():
		return transport.Message{}, ctx.Err()
	case <-e.done:
		return transport.Message{}, transport.ErrClosed
	case <-e.peer.done:
		return transport.Message{}, transport.ErrClosed
	}
}

func (e *end) Close() error {
	e.closed.Do(func() { close(e.done) })
	return nil
}

func (e *end) rollLocked() int32 { return int32(e.rng.Int64n(100)) }

func (e *end) delayLocked() time.Duration {
	d := e.f.Latency
	if e.f.Jitter > 0 {
		d += time.Duration(e.rng.Int64n(int64(e.f.Jitter)))
	}
	return d
}

func clone(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
