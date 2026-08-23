// Package netplay wires a session across real transports: the server side
// admits connections, turns broadcasts into per-receiver state streams,
// polices inbound traffic, and detects departures; the client side
// reconstructs the world and drives an agent. It is generic over the
// game's state S, action A, generated delta D, and sight O — the
// pong and tron samples share every line of it.
//
// Enforcement lives here because this is the authoritative boundary:
// permission:spectator-receive-only (a spectator's inputs are violations,
// not messages), policy:realtime-abuse-protection (rate limits, malformed
// input, escalation to disconnect), policy:overload-handling (bounded
// queues, shed-then-snapshot), and concept:agent-departure-policy
// (detection by transport close or silence deadline; what happens next is
// the game's callback).
package netplay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/budget"
	"github.com/shibukawa/ebigentserver/observe"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/seqack"
)

// control is the reliable-channel envelope shared with the client side.
type control struct {
	T string `json:"t"`
}

// RoleSpectator is the ticket role admitted without a seat
// (permission:spectator-receive-only).
const RoleSpectator = "spectator"

// abuseDisconnectThreshold closes a connection after this many protocol
// violations (spectator inputs, malformed datagrams, rate excess) — the
// final rung of the escalation ladder.
const abuseDisconnectThreshold = 32

// ServerConfig assembles the server side for one session.
type ServerConfig[S, A any] struct {
	// SessionID names events.
	SessionID string
	// Protocol is the exact wire version (rule:protocol-version-must-match).
	Protocol string
	// Verifier checks tickets locally.
	Verifier *admission.Verifier
	// Seed travels in the Welcome (rule:shared-rng-seed).
	Seed uint64
	// Tuning is the declared profile; Budget the declared ceilings.
	Tuning session.TuningProfile
	Budget budget.Budget
	// MakeSender builds the outbound pipeline for one admitted
	// receiver. Global-visibility games return a plain statesync.Sender;
	// scoped games return a statesync.ProjectedSender whose projection
	// depends on the seat and role — this is where concept:agent-view
	// attaches, and why hidden state never reaches serialization.
	MakeSender func(slot session.SlotID, role string) (statesync.ViewSender[S], error)
	// DecodeInput parses one wire-profile input payload.
	DecodeInput func([]byte) (A, error)
	// Inbox resolves a seat's mailbox — pass the session's Inbox method.
	Inbox func(session.SlotID) (*session.Inbox[A], error)
	// OnDeparture fires when a connection is declared lost (transport
	// close or silence deadline). The game chooses what happens to the
	// seat (concept:agent-departure-policy): abort the session, play
	// on, or seat a takeover bot — every option is external because a
	// seat accepts any agent already.
	OnDeparture func(slot session.SlotID, role string, reason string)
	// Metrics and Events receive evidence; nil drops it.
	Metrics *observe.Metrics
	Events  *observe.Log
}

// Server owns the admitted peers.
type Server[S, A any] struct {
	cfg     ServerConfig[S, A]
	ctx     context.Context
	mu      sync.Mutex
	peers   []*Peer[S, A]
	pending int32 // handshakes in flight; they hold capacity too
}

// NewServer validates the declarations and builds the server.
func NewServer[S, A any](ctx context.Context, cfg ServerConfig[S, A]) (*Server[S, A], error) {
	if err := cfg.Tuning.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.Budget.Validate(); err != nil {
		return nil, err
	}
	if cfg.DecodeInput == nil || cfg.Inbox == nil || cfg.Verifier == nil || cfg.MakeSender == nil {
		return nil, errors.New("netplay: DecodeInput, Inbox, Verifier, and MakeSender are required")
	}
	if cfg.Metrics == nil {
		cfg.Metrics = &observe.Metrics{} // unobserved but never nil
	}
	return &Server[S, A]{cfg: cfg, ctx: ctx}, nil
}

// Admit runs the handshake on a fresh connection and registers the peer.
// Capacity exhaustion fails closed before any allocation
// (policy:realtime-abuse-protection).
func (sv *Server[S, A]) Admit(ctx context.Context, conn transport.Conn) (*Peer[S, A], error) {
	// A handshake in flight holds capacity, so a burst of half-open
	// connections cannot exceed the declared ceiling.
	sv.mu.Lock()
	over := int32(len(sv.peers))+sv.pending >= sv.cfg.Budget.MaxConnections
	if !over {
		sv.pending++
	}
	sv.mu.Unlock()
	if over {
		sv.cfg.Metrics.AdmissionRejected.Add(1)
		sv.event(observe.Event{Kind: "admission_reject", Reason: "capacity"})
		_ = conn.Close()
		return nil, fmt.Errorf("netplay: connection capacity exhausted")
	}
	defer func() {
		sv.mu.Lock()
		sv.pending--
		sv.mu.Unlock()
	}()
	claims, err := admission.Accept(ctx, conn, sv.cfg.Protocol, sv.cfg.Verifier, sv.cfg.Seed)
	if err != nil {
		sv.cfg.Metrics.AdmissionRejected.Add(1)
		sv.event(observe.Event{Kind: "admission_reject", Reason: err.Error()})
		return nil, err
	}
	p := &Peer[S, A]{
		sv:    sv,
		conn:  conn,
		Slot:  session.SlotID(claims.Seat),
		Role:  claims.Role,
		layer: seqack.New(conn, seqack.Options{Policy: seqack.PiggybackOnly}),
	}
	if p.Role != RoleSpectator {
		inbox, err := sv.cfg.Inbox(p.Slot)
		if err != nil {
			sv.event(observe.Event{Kind: "admission_reject", Slot: uint16(claims.Seat), Reason: "unknown_seat"})
			_ = conn.Close()
			return nil, fmt.Errorf("netplay: ticket names unknown seat %d: %w", claims.Seat, err)
		}
		p.inbox = inbox
	}
	snd, err := sv.cfg.MakeSender(p.Slot, p.Role)
	if err != nil {
		return nil, err
	}
	p.sender = snd
	// Token bucket sized from the declared per-tick input bound.
	p.inputBucket = float64(sv.cfg.Budget.InputsPerTick)
	sv.mu.Lock()
	sv.peers = append(sv.peers, p)
	sv.mu.Unlock()
	sv.cfg.Metrics.ActiveConnections.Add(1)
	return p, nil
}

// Broadcast matches session.Config.Broadcast: encode and send the
// committed world to every live peer, and sweep for silent ones.
func (sv *Server[S, A]) Broadcast(tick session.Tick, world *S) {
	sv.cfg.Metrics.TicksCommitted.Add(1)
	deadline := time.Duration(0)
	if sv.cfg.Tuning.SilenceDeadline > 0 {
		deadline = time.Duration(sv.cfg.Tuning.SilenceDeadline) * time.Second / time.Duration(sv.cfg.Tuning.TickRate)
	}
	sv.mu.Lock()
	peers := append([]*Peer[S, A](nil), sv.peers...)
	sv.mu.Unlock()
	now := time.Now()
	for _, p := range peers {
		if p.gone.Load() {
			continue
		}
		p.sendState(sv.ctx, tick, world)
		if deadline > 0 {
			if last := p.layer.Stats().LastReceived; !last.IsZero() && now.Sub(last) > deadline {
				p.depart("silence_deadline")
			}
		}
	}
}

// RetainedCost measures the per-receiver baseline retention of
// decision:framework-side-delta-generation: receivers times retained
// versions — the material for judging history_depth (Phase 4
// acceptance); callers size one version with their own codec.
func (sv *Server[S, A]) RetainedCost() (receivers int, versionsPerReceiver int32) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	return len(sv.peers), sv.cfg.Tuning.HistoryDepth
}

func (sv *Server[S, A]) event(ev observe.Event) {
	ev.SessionID = sv.cfg.SessionID
	sv.cfg.Events.Emit(ev)
}

func (sv *Server[S, A]) removePeer(p *Peer[S, A]) {
	sv.mu.Lock()
	for i, q := range sv.peers {
		if q == p {
			sv.peers = append(sv.peers[:i], sv.peers[i+1:]...)
			break
		}
	}
	sv.mu.Unlock()
}
