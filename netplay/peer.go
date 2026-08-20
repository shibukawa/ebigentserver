package netplay

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shibukawa/ebigentserver/observe"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/seqack"
)

// Peer is one admitted connection on the server side.
type Peer[S, A any] struct {
	sv   *Server[S, A]
	conn transport.Conn
	// Slot is the seat; spectators keep the ticket's seat value for
	// identification but hold no inbox.
	Slot session.SlotID
	// Role is the admitted role claim.
	Role string

	layer *seqack.Layer
	inbox *session.Inbox[A] // nil for spectators

	mu     sync.Mutex
	sender statesync.ViewSender[S]

	// abuse accounting (policy:realtime-abuse-protection)
	violations   atomic.Int32
	inputBucket  float64
	bucketFilled time.Time

	gone     atomic.Bool
	departed sync.Once
}

// sendState encodes and sends one committed world version.
func (p *Peer[S, A]) sendState(ctx context.Context, tick session.Tick, world *S) {
	p.mu.Lock()
	if conf, ok := p.layer.Confirmed(); ok {
		p.sender.Confirm(session.Tick(conf))
	}
	pkt := p.sender.Send(tick, world)
	p.mu.Unlock()
	_ = p.layer.SendDatagram(ctx, pkt.AppendWire(nil), uint64(tick))
}

// Run consumes the connection until it closes, then reports the
// departure. Run it on its own goroutine.
func (p *Peer[S, A]) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer p.depart("transport_closed")
	for {
		m, err := p.conn.Receive(ctx)
		if err != nil {
			return
		}
		switch m.Channel {
		case transport.Unreliable:
			p.absorbDatagram(m.Payload)
		case transport.Reliable:
			var c control
			if json.Unmarshal(m.Payload, &c) == nil && c.T == "resync" {
				p.sv.cfg.Metrics.ResyncRequests.Add(1)
				p.mu.Lock()
				p.sender.ResyncRequested()
				p.mu.Unlock()
			}
		}
		if p.violations.Load() >= abuseDisconnectThreshold {
			p.sv.cfg.Metrics.Disconnects.Add(1)
			p.sv.event(observe.Event{Kind: "disconnect", Slot: uint16(p.Slot), Reason: "abuse_threshold"})
			_ = p.conn.Close()
			return
		}
	}
}

func (p *Peer[S, A]) absorbDatagram(payload []byte) {
	data := p.layer.Absorb(payload)
	if data == nil {
		return // ack-only, stale, or not ours
	}
	// A spectator submits no actions: any input datagram is a protocol
	// violation, never a message (permission:spectator-receive-only).
	if p.inbox == nil {
		p.violate("spectator_input")
		return
	}
	if !p.takeToken() {
		p.violate("input_rate")
		return
	}
	in, err := p.sv.cfg.DecodeInput(data)
	if err != nil {
		p.violate("malformed_input")
		return
	}
	p.sv.cfg.Metrics.InputsAccepted.Add(1)
	p.inbox.Submit(in)
}

// takeToken enforces the declared input rate: a bucket of
// Budget.InputsPerTick tokens refilled at InputsPerTick per tick
// duration.
func (p *Peer[S, A]) takeToken() bool {
	now := time.Now()
	perTick := float64(p.sv.cfg.Budget.InputsPerTick)
	tickDur := time.Second / time.Duration(p.sv.cfg.Tuning.TickRate)
	if !p.bucketFilled.IsZero() {
		p.inputBucket += now.Sub(p.bucketFilled).Seconds() / tickDur.Seconds() * perTick
	}
	if cap := perTick * 4; p.inputBucket > cap {
		p.inputBucket = cap
	}
	p.bucketFilled = now
	if p.inputBucket < 1 {
		return false
	}
	p.inputBucket--
	return true
}

func (p *Peer[S, A]) violate(reason string) {
	p.violations.Add(1)
	p.sv.cfg.Metrics.InputsRejected.Add(1)
	p.sv.event(observe.Event{Kind: "abuse_reject", Slot: uint16(p.Slot), Reason: reason})
}

// depart declares the connection lost exactly once and hands the seat to
// the game's departure policy.
func (p *Peer[S, A]) depart(reason string) {
	p.departed.Do(func() {
		p.gone.Store(true)
		p.sv.cfg.Metrics.ActiveConnections.Add(-1)
		p.sv.removePeer(p)
		p.sv.event(observe.Event{Kind: "departure", Slot: uint16(p.Slot), Reason: reason})
		if p.sv.cfg.OnDeparture != nil {
			p.sv.cfg.OnDeparture(p.Slot, p.Role, reason)
		}
	})
}
