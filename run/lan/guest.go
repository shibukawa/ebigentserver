//go:build !js && !wasip1

package lan

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// This file is the guest's side of run.Joined: the small amount of state
// that turns a link into something a play scene can treat exactly like a
// local match.

// mailbox carries one action from the frame goroutine to the link
// goroutine. The frame produces at the display's rate and the link
// consumes at the send rate, so the newest action wins — which is the
// same intake policy a local seat gets under IntakeNewest.
type mailbox[A any] struct {
	mu     sync.Mutex
	action A
	filled bool
}

func (m *mailbox[A]) put(a A) {
	m.mu.Lock()
	m.action, m.filled = a, true
	m.mu.Unlock()
}

func (m *mailbox[A]) take() (A, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.action, m.filled
	m.filled = false
	return a, ok
}

// local is the guest's own controller, and it is an ordinary agent. A
// bot put in its place would change nothing else, which is the property
// decision:agent-as-central-abstraction is for.
type local[S, A, D, O any] struct {
	g *Guest[S, A, D, O]
}

func (l *local[S, A, D, O]) Joined(session.SlotID) {}

// Observe runs on the link goroutine, immediately after the receiver
// reconstructed this world — which is the only place it is safe to read.
func (l *local[S, A, D, O]) Observe(O) {
	sink := l.g.sink()
	if sink == nil {
		return
	}
	if world, tick, ok := l.g.State(); ok {
		sink(tick, world)
	}
}

func (l *local[S, A, D, O]) Decide(context.Context) (A, bool) { return l.g.box.take() }

func (l *local[S, A, D, O]) Ended(session.Result) {}

// LocalSeats reports the one seat this machine plays. It is known from
// the seat grant, so a guest can draw and take input while the handshake
// is still waiting on the host.
func (g *Guest[S, A, D, O]) LocalSeats() []run.Seat {
	return []run.Seat{{Slot: g.seat, Kind: run.Human, Local: true, ID: "you", Ready: true}}
}

// Submit hands this frame's action to the link.
func (g *Guest[S, A, D, O]) Submit(slot session.SlotID, action A) error {
	if slot != g.seat {
		return run.ErrUnknownSlot
	}
	g.box.put(action)
	return nil
}

// OnWorld registers the sink for each reconstructed world.
func (g *Guest[S, A, D, O]) OnWorld(f func(session.Tick, *S)) {
	g.mu.Lock()
	g.onWorld = f
	g.mu.Unlock()
}

func (g *Guest[S, A, D, O]) sink() func(session.Tick, *S) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.onWorld
}

// Play drives the link with the guest's own controller.
func (g *Guest[S, A, D, O]) Play(ctx context.Context) error {
	err := g.Run(ctx, &local[S, A, D, O]{g: g})
	g.mu.Lock()
	g.over = true
	g.mu.Unlock()
	return err
}

// Over reports that Play has returned — the match ended, or the host did.
func (g *Guest[S, A, D, O]) Over() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.over
}

// Preset is what a game hands to the wrapper: this network, this
// protocol, these wire types. The wrapper asks it what is out there and
// the player decides — which is why hosting and joining are separate
// calls rather than one that picks for them.
func Preset[S, A, D, O any](opts Options[S, A, D, O]) run.Networking[S, A, O] {
	return &preset[S, A, D, O]{opts: opts, window: opts.browseWindow()}
}

type preset[S, A, D, O any] struct {
	opts   Options[S, A, D, O]
	window time.Duration
}

// Discover lists the games answering on this network.
func (p *preset[S, A, D, O]) Discover(ctx context.Context) ([]run.Found, error) {
	beacons, err := Browse(ctx, p.opts, p.window)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	out := make([]run.Found, 0, len(beacons))
	for _, b := range beacons {
		out = append(out, run.Found{Name: b.Session, Address: b.Endpoint, Players: b.PlayerCount})
	}
	return out, nil
}

// Host offers this instance's match.
func (p *preset[S, A, D, O]) Host(ctx context.Context, r *run.Roster[S, A, O], seed uint64) (run.Hosting[S, A, O], error) {
	return Open(ctx, p.opts, r, seed)
}

// Join takes a seat on one of the games Discover reported.
func (p *preset[S, A, D, O]) Join(ctx context.Context, f run.Found) (run.Joined[S, A, O], error) {
	return JoinAt(ctx, p.opts, f.Address)
}

// The two halves of run.Networking, checked here so a signature drift in
// either package fails at build time rather than at a player's keyboard.
var (
	_ run.Hosting[int, int, int] = (*Host[int, int, int, int])(nil)
	_ run.Joined[int, int, int]  = (*Guest[int, int, int, int])(nil)
)
