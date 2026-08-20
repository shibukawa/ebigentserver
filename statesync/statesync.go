// Package statesync implements decision:framework-side-delta-generation:
// the framework retains committed world versions per receiver and computes
// data:state-delta by diffing them; games declare struct types and scales
// and never hand-write delta code. The diff and codec functions plug in
// from tinybind-generated code.
//
// Phase 3a scope: the speculative baseline of concept:delta-baseline-policy
// (baseline = last sent version) over a loss-free local link, with the
// full-snapshot fallback of rule:delta-baseline-must-be-retained. Ack
// tracking and the remaining baseline modes arrive with Phase 3b's
// api:sequence-ack-layer.
package statesync

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/shibukawa/ebigentserver/session"
)

// Kind tags a packet.
type Kind uint8

const (
	// KindSnapshot is a full data:snapshot.
	KindSnapshot Kind = iota + 1
	// KindDelta is a data:state-delta against a named baseline.
	KindDelta
)

// Packet is one downstream state update. Baseline names the version a
// delta was computed from (rule:delta-baseline-must-be-retained); a
// receiver that does not hold it must reject the packet and resync.
type Packet struct {
	Kind     Kind
	Tick     session.Tick
	Baseline session.Tick // deltas only
	Payload  []byte
}

// AppendWire encodes the packet for transit: kind(1), tick(8),
// baseline(8), payload. The layout is frozen; it travels inside the
// datagrams of api:sequence-ack-layer.
func (p Packet) AppendWire(dst []byte) []byte {
	dst = append(dst, byte(p.Kind))
	dst = binary.BigEndian.AppendUint64(dst, uint64(p.Tick))
	dst = binary.BigEndian.AppendUint64(dst, uint64(p.Baseline))
	return append(dst, p.Payload...)
}

// DecodeWire parses one transit-encoded packet; the payload borrows from
// data.
func DecodeWire(data []byte) (Packet, error) {
	if len(data) < 17 {
		return Packet{}, errors.New("statesync: short packet")
	}
	return Packet{
		Kind:     Kind(data[0]),
		Tick:     session.Tick(binary.BigEndian.Uint64(data[1:9])),
		Baseline: session.Tick(binary.BigEndian.Uint64(data[9:17])),
		Payload:  data[17:],
	}, nil
}

// Codec wires a game's tinybind-generated functions into the framework.
// S is the world state, D the generated delta type.
type Codec[S, D any] struct {
	// AppendSnapshot encodes the full state (world profile).
	AppendSnapshot func(dst []byte, s *S) []byte
	// DecodeSnapshot decodes a full state.
	DecodeSnapshot func(s *S, data []byte) error
	// Diff computes the delta from baseline to current.
	Diff func(baseline, current *S) D
	// AppendDelta encodes a delta.
	AppendDelta func(dst []byte, d *D) []byte
	// DecodeDelta decodes a delta.
	DecodeDelta func(d *D, data []byte) error
	// ApplyDelta applies a delta to a state.
	ApplyDelta func(s *S, d D) error
	// Clone copies a state for retention. Nil uses a value copy, which
	// is correct only for states without reference fields — a state
	// holding slices must supply a deep copy (the shallow-baseline trap
	// is exactly what made every diff empty once already).
	Clone func(s *S) S
}

func (c Codec[S, D]) clone(s *S) S {
	if c.Clone != nil {
		return c.Clone(s)
	}
	return *s
}

func (c Codec[S, D]) validate() error {
	if c.AppendSnapshot == nil || c.DecodeSnapshot == nil || c.Diff == nil ||
		c.AppendDelta == nil || c.DecodeDelta == nil || c.ApplyDelta == nil {
		return errors.New("statesync: every Codec function except Clone is required")
	}
	return nil
}

// retained is one kept world version.
type retained[S any] struct {
	tick  session.Tick
	state S
}

// Sender produces the packet stream for one receiver. Retention is per
// receiver (decision:framework-side-delta-generation's cost note), so a
// session runs one Sender per receiver.
type Sender[S, D any] struct {
	codec     Codec[S, D]
	history   []retained[S] // newest last, bounded by HistoryDepth
	depth     int32
	snapEvery int32 // full snapshot every N-th send; 0 = only on resync
	sends     int32 // sends since the last snapshot
	lastSent  session.Tick
	started   bool
	needSnap  bool

	mode        session.BaselineMode
	specDepth   int32
	confirmed   session.Tick
	confirmedOK bool
}

// NewSender builds a sender for one receiver from the declared tuning
// profile.
func NewSender[S, D any](codec Codec[S, D], tuning session.TuningProfile) (*Sender[S, D], error) {
	if err := codec.validate(); err != nil {
		return nil, err
	}
	if err := tuning.Validate(); err != nil {
		return nil, err
	}
	return &Sender[S, D]{
		codec:     codec,
		depth:     tuning.HistoryDepth,
		snapEvery: tuning.SnapshotEvery,
		mode:      tuning.BaselineMode,
		specDepth: tuning.SpeculationDepth,
	}, nil
}

// ResyncRequested forces the next packet to be a full snapshot — the
// receiver reported a baseline it does not hold.
func (s *Sender[S, D]) ResyncRequested() { s.needSnap = true }

// Confirm records the newest version the peer is known to hold, as
// reported by api:sequence-ack-layer. The confirmed and bounded baseline
// modes diff against it.
func (s *Sender[S, D]) Confirm(tick session.Tick) {
	if !s.confirmedOK || tick > s.confirmed {
		s.confirmed, s.confirmedOK = tick, true
	}
}

// baseline picks the version to diff against per the declared mode
// (concept:delta-baseline-policy); ok=false forces a snapshot.
func (s *Sender[S, D]) baseline(tick session.Tick) (session.Tick, bool) {
	switch s.mode {
	case session.BaselineConfirmedOnly:
		return s.confirmed, s.confirmedOK
	case session.BaselineBounded:
		if s.confirmedOK && tick-s.confirmed <= session.Tick(s.specDepth) {
			return s.lastSent, true
		}
		return s.confirmed, s.confirmedOK
	default: // BaselineSpeculative
		return s.lastSent, true
	}
}

// Send produces the packet for the committed world at tick. The first
// send is always a snapshot (it is what carries the joining receiver's
// baseline); later sends are deltas against the last sent version unless
// the snapshot cadence or a resync forces a full state.
func (s *Sender[S, D]) Send(tick session.Tick, world *S) Packet {
	s.retain(tick, world)
	snap := !s.started || s.needSnap
	if !snap && s.snapEvery > 0 && s.sends >= s.snapEvery {
		snap = true
	}
	var base *retained[S]
	if !snap {
		baseTick, ok := s.baseline(tick)
		if base = s.find(baseTick); !ok || base == nil {
			// No usable baseline, or it aged out of retention: forced
			// snapshot rather than an unbounded buffer
			// (rule:delta-baseline-must-be-retained).
			snap = true
		}
	}
	var pkt Packet
	if snap {
		pkt = Packet{Kind: KindSnapshot, Tick: tick, Payload: s.codec.AppendSnapshot(nil, world)}
		s.sends = 0
		s.needSnap = false
	} else {
		d := s.codec.Diff(&base.state, world)
		pkt = Packet{Kind: KindDelta, Tick: tick, Baseline: base.tick, Payload: s.codec.AppendDelta(nil, &d)}
		s.sends++
	}
	s.started = true
	s.lastSent = tick
	return pkt
}

func (s *Sender[S, D]) retain(tick session.Tick, world *S) {
	s.history = append(s.history, retained[S]{tick: tick, state: s.codec.clone(world)})
	if int32(len(s.history)) > s.depth {
		s.history = s.history[1:]
	}
}

func (s *Sender[S, D]) find(tick session.Tick) *retained[S] {
	for i := range s.history {
		if s.history[i].tick == tick {
			return &s.history[i]
		}
	}
	return nil
}

// ErrResyncNeeded reports a delta whose baseline the receiver does not
// hold; the receiver must request a snapshot (Sender.ResyncRequested).
var ErrResyncNeeded = errors.New("statesync: delta baseline not held, resync needed")

// Receiver reconstructs the world from the packet stream. It retains its
// own recent versions: under an acked link the sender may legitimately
// diff against any version the receiver reported holding, not only the
// newest one it happens to have.
type Receiver[S, D any] struct {
	codec   Codec[S, D]
	history []retained[S] // newest last
	depth   int32
	synced  bool
}

// NewReceiver builds a receiver retaining up to the tuning profile's
// history depth.
func NewReceiver[S, D any](codec Codec[S, D], tuning session.TuningProfile) (*Receiver[S, D], error) {
	if err := codec.validate(); err != nil {
		return nil, err
	}
	if err := tuning.Validate(); err != nil {
		return nil, err
	}
	return &Receiver[S, D]{codec: codec, depth: tuning.HistoryDepth}, nil
}

// Apply consumes one packet. A delta against a version the receiver does
// not hold returns ErrResyncNeeded and changes nothing; a packet older
// than the newest applied one is ignored (the state stream is
// last-writer-wins).
func (r *Receiver[S, D]) Apply(pkt Packet) error {
	if r.synced && pkt.Tick <= r.newest().tick {
		return nil // stale arrival
	}
	switch pkt.Kind {
	case KindSnapshot:
		var s S
		if err := r.codec.DecodeSnapshot(&s, pkt.Payload); err != nil {
			return fmt.Errorf("statesync: snapshot at tick %d: %w", pkt.Tick, err)
		}
		r.push(pkt.Tick, s)
		r.synced = true
		return nil
	case KindDelta:
		if !r.synced {
			return ErrResyncNeeded
		}
		base := r.find(pkt.Baseline)
		if base == nil {
			return ErrResyncNeeded
		}
		next := r.codec.clone(&base.state)
		var d D
		if err := r.codec.DecodeDelta(&d, pkt.Payload); err != nil {
			return fmt.Errorf("statesync: delta at tick %d: %w", pkt.Tick, err)
		}
		if err := r.codec.ApplyDelta(&next, d); err != nil {
			return fmt.Errorf("statesync: apply delta at tick %d: %w", pkt.Tick, err)
		}
		r.push(pkt.Tick, next)
		return nil
	default:
		return fmt.Errorf("statesync: unknown packet kind %d", pkt.Kind)
	}
}

func (r *Receiver[S, D]) push(tick session.Tick, s S) {
	r.history = append(r.history, retained[S]{tick: tick, state: s})
	if int32(len(r.history)) > r.depth {
		r.history = r.history[1:]
	}
}

func (r *Receiver[S, D]) newest() *retained[S] { return &r.history[len(r.history)-1] }

func (r *Receiver[S, D]) find(tick session.Tick) *retained[S] {
	for i := range r.history {
		if r.history[i].tick == tick {
			return &r.history[i]
		}
	}
	return nil
}

// State returns the newest reconstructed world and its tick; ok is false
// until the first snapshot lands.
func (r *Receiver[S, D]) State() (world *S, tick session.Tick, ok bool) {
	if !r.synced {
		var zero S
		return &zero, 0, false
	}
	n := r.newest()
	return &n.state, n.tick, true
}
