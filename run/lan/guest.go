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

// This file is the guest's side of run.Guest: the small amount of state
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
type local[W, A, D, S any] struct {
	g *Guest[W, A, D, S]
}

func (l *local[W, A, D, S]) Joined(session.SlotID) {}

// Observe runs on the link goroutine, immediately after the receiver
// reconstructed this world — which is the only place it is safe to read.
func (l *local[W, A, D, S]) Observe(S) {
	sink := l.g.sink()
	if sink == nil {
		return
	}
	if world, tick, ok := l.g.State(); ok {
		sink(tick, world)
	}
}

func (l *local[W, A, D, S]) Decide(context.Context) (A, bool) { return l.g.box.take() }

func (l *local[W, A, D, S]) Ended(session.Result) {}

// LocalSeats reports the one seat this machine plays. It is known from
// the seat grant, so a guest can draw and take input while the handshake
// is still waiting on the host — and it is empty before Sit, because an
// instance that only matched plays nothing.
func (g *Guest[W, A, D, S]) LocalSeats() []run.Seat {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.seated {
		return nil
	}
	return []run.Seat{{Slot: g.seat, Kind: run.Human, Local: true, ID: "you", Ready: true}}
}

// Submit hands this frame's action to the link.
func (g *Guest[W, A, D, S]) Submit(slot session.SlotID, action A) error {
	g.mu.Lock()
	seat, seated := g.seat, g.seated
	g.mu.Unlock()
	if !seated || slot != seat {
		return run.ErrUnknownSlot
	}
	g.box.put(action)
	return nil
}

// OnWorld registers the sink for each reconstructed world.
func (g *Guest[W, A, D, S]) OnWorld(f func(session.Tick, *W)) {
	g.mu.Lock()
	g.onWorld = f
	g.mu.Unlock()
}

func (g *Guest[W, A, D, S]) sink() func(session.Tick, *W) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.onWorld
}

// Play drives the link with the guest's own controller. It needs a seat:
// matching alone opens nothing to drive.
func (g *Guest[W, A, D, S]) Play(ctx context.Context) error {
	if !g.Seated() {
		return errors.New("lan: Play before Sit; this instance holds no seat")
	}
	err := g.Run(ctx, &local[W, A, D, S]{g: g})
	g.mu.Lock()
	g.over = true
	g.mu.Unlock()
	return err
}

// Over reports that Play has returned — the match ended, or the host did.
func (g *Guest[W, A, D, S]) Over() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.over
}

// Preset is what a game hands to the wrapper: this network, this
// protocol, these wire types. The wrapper asks it what is out there and
// the player decides — which is why hosting and joining are separate
// calls rather than one that picks for them.
func Preset[W, A, D, S any](opts Options[W, A, D, S]) run.Matchmaking[W, A, S] {
	return &preset[W, A, D, S]{opts: opts, window: opts.browseWindow()}
}

type preset[W, A, D, S any] struct {
	opts   Options[W, A, D, S]
	window time.Duration
}

// Discover lists the games answering on this network.
func (p *preset[W, A, D, S]) Discover(ctx context.Context) ([]run.Found, error) {
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
func (p *preset[W, A, D, S]) Host(ctx context.Context, r *run.Roster[W, A, S], seed uint64) (run.Host[W, A, S], error) {
	return Open(ctx, p.opts, r, seed)
}

// Match reaches one of the rooms Discover reported. No seat is taken;
// that is Guest.Sit.
func (p *preset[W, A, D, S]) Match(ctx context.Context, f run.Found) (run.Guest[W, A, S], error) {
	return MatchAt(ctx, p.opts, f.Address)
}

// The two halves of run.Matchmaking, checked here so a signature drift in
// either package fails at build time rather than at a player's keyboard.
var (
	_ run.Host[int, int, int]  = (*Host[int, int, int, int])(nil)
	_ run.Guest[int, int, int] = (*Guest[int, int, int, int])(nil)
)
