package statesync

import (
	"fmt"
	"sync"

	"github.com/shibukawa/ebigentserver/session"
)

// Hub fans a session's committed world out to per-receiver packet
// channels over the in-process loopback link. One Sender lives per
// receiver, because retention is per receiver.
//
// Hub.Broadcast has the session.Config.Broadcast signature: the session
// stays transport-blind and the hub owns delivery. A receiver whose
// channel is full loses the packet; the hub then forces that receiver's
// next send to be a snapshot, since a lost packet breaks the speculative
// delta chain (concept:delta-baseline-policy).
type Hub[S, D any] struct {
	codec  Codec[S, D]
	tuning session.TuningProfile

	mu      sync.Mutex
	slots   []session.SlotID
	senders map[session.SlotID]*Sender[S, D]
	chans   map[session.SlotID]chan Packet
	closed  bool
}

// chanDepth buffers a few sends per receiver before drops begin.
const chanDepth = 16

// NewHub builds a hub for one session.
func NewHub[S, D any](codec Codec[S, D], tuning session.TuningProfile) (*Hub[S, D], error) {
	if err := codec.validate(); err != nil {
		return nil, err
	}
	if err := tuning.Validate(); err != nil {
		return nil, err
	}
	return &Hub[S, D]{
		codec:   codec,
		tuning:  tuning,
		senders: map[session.SlotID]*Sender[S, D]{},
		chans:   map[session.SlotID]chan Packet{},
	}, nil
}

// Attach registers a receiver and returns its packet channel. Attach every
// receiver before the session starts broadcasting.
func (h *Hub[S, D]) Attach(slot session.SlotID) (<-chan Packet, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, dup := h.senders[slot]; dup {
		return nil, fmt.Errorf("statesync: slot %d already attached", slot)
	}
	snd, err := NewSender(h.codec, h.tuning)
	if err != nil {
		return nil, err
	}
	ch := make(chan Packet, chanDepth)
	h.senders[slot] = snd
	h.chans[slot] = ch
	h.slots = append(h.slots, slot)
	return ch, nil
}

// RequestResync asks for a full snapshot on the receiver's next send
// (Receiver returned ErrResyncNeeded).
func (h *Hub[S, D]) RequestResync(slot session.SlotID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if snd, ok := h.senders[slot]; ok {
		snd.ResyncRequested()
	}
}

// Broadcast encodes and delivers the committed world to every receiver.
// It matches session.Config.Broadcast.
func (h *Hub[S, D]) Broadcast(tick session.Tick, world *S) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	for _, slot := range h.slots {
		snd := h.senders[slot]
		pkt := snd.Send(tick, world)
		select {
		case h.chans[slot] <- pkt:
		default:
			// Receiver is behind: the packet is lost, so the delta
			// chain is broken for it. Snapshot next time.
			snd.ResyncRequested()
		}
	}
}

// Close closes every receiver channel; call it once the session is done.
func (h *Hub[S, D]) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, ch := range h.chans {
		close(ch)
	}
}
