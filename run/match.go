package run

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/shibukawa/ebigentserver/session"
)

// Match is one running game: the concept:session a finalized api:roster
// produced, plus the seats that fill it and the mailboxes their actions
// travel through. It is the running state of concept:match-lifecycle.
type Match[S, A, O any] struct {
	opts    Options
	seats   []Seat
	sess    *session.Session[S, A, O]
	inboxes map[session.SlotID]*session.Inbox[A]
	drivers []*driver[S, A, O]

	mu   sync.Mutex
	err  error
	done chan struct{}
}

// driver pumps one local controller. A realtime session never calls
// Observe or Decide itself — input arrives through the slot's inbox,
// because that is the path a remote peer uses and the framework refuses
// to keep two (rule:session-independent-of-transport-and-agent-kind). A
// local agent therefore needs something to turn each committed world into
// an observation, ask for a decision, and submit it. This is that.
type driver[S, A, O any] struct {
	slot  session.SlotID
	agent session.Agent[O, A]
	inbox *session.Inbox[A]
	game  session.StageRuleSet[S, A, O]
	ctx   context.Context
}

// pump runs one controller against one committed world. It runs on the
// session's goroutine, so a controller that blocks here stalls the tick.
func (d *driver[S, A, O]) pump(world *S) {
	obs := d.game.Project(world, d.slot)
	d.agent.Observe(obs)
	ctx := d.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if action, ok := d.agent.Decide(ctx); ok {
		d.inbox.Submit(action)
	}
}

// Finalize freezes the roster into a running match: it builds the
// session, seats every agent, and returns before the first tick. Play
// starts at Run or Start.
//
// cfg is the game's own session configuration — its rules, validator,
// tuning, seed, and canonical encoding. The wrapper adds exactly two
// things: the local controller pump on the broadcast seam, and whatever
// recorder Options and data:run-config asked for. Anything the game set
// on cfg.Broadcast still runs, after the pump.
func (r *Roster[S, A, O]) Finalize(cfg session.Config[S, A, O]) (*Match[S, A, O], error) {
	if !r.Complete() {
		return nil, fmt.Errorf("%w: %s", ErrIncomplete, describeEmpty(r.Seats()))
	}
	declared := slices.Clone(cfg.Slots)
	slices.Sort(declared)
	if !slotsEqual(declared, r.slots) {
		return nil, fmt.Errorf("run: roster seats %v but the session config declares %v", r.slots, declared)
	}
	if cfg.ID == "" {
		cfg.ID = r.opts.Name
	}

	m := &Match[S, A, O]{
		opts:    r.opts,
		seats:   r.Seats(),
		inboxes: make(map[session.SlotID]*session.Inbox[A], len(r.slots)),
		done:    make(chan struct{}),
	}

	game := cfg.RuleSet
	appBroadcast := cfg.Broadcast
	cfg.Broadcast = func(tick session.Tick, world *S) {
		// Local controllers decide against the world that just
		// committed; their actions land in the next tick's intake.
		for _, d := range m.drivers {
			d.pump(world)
		}
		if appBroadcast != nil {
			appBroadcast(tick, world)
		}
	}

	sess, err := session.New(cfg)
	if err != nil {
		return nil, err
	}
	if err := sess.OpenAdmission(); err != nil {
		return nil, err
	}
	m.sess = sess

	r.mu.Lock()
	agents := make(map[session.SlotID]session.Agent[O, A], len(r.agents))
	for slot, agent := range r.agents {
		agents[slot] = agent
	}
	r.mu.Unlock()

	for _, seat := range m.seats {
		agent := agents[seat.Slot]
		if agent == nil {
			// A human seat, local or remote: the session sees a
			// detached slot and reads the inbox, so the local
			// path and the network path stay the same path.
			agent = session.Detached[O, A]{}
		}
		if err := sess.Admit(seat.Slot, agent); err != nil {
			return nil, err
		}
		inbox, err := sess.Inbox(seat.Slot)
		if err != nil {
			return nil, err
		}
		m.inboxes[seat.Slot] = inbox
		if seat.LocalBot() {
			m.drivers = append(m.drivers, &driver[S, A, O]{
				slot:  seat.Slot,
				agent: agent,
				inbox: inbox,
				game:  game,
			})
		}
	}
	return m, nil
}

// describeEmpty names the unfilled seats for the incomplete error.
func describeEmpty(seats []Seat) string {
	var empty []session.SlotID
	for _, seat := range seats {
		if !seat.Filled() {
			empty = append(empty, seat.Slot)
		}
	}
	return fmt.Sprintf("slots %v", empty)
}

// Session exposes the running session for callers that need it directly —
// tick counts, lifecycle state, and the seams netplay attaches to.
func (m *Match[S, A, O]) Session() *session.Session[S, A, O] { return m.sess }

// Seats reports the roster this match was built from.
func (m *Match[S, A, O]) Seats() []Seat { return slices.Clone(m.seats) }

// LocalSeats reports the seats a person at this machine controls, in slot
// order. The intake hook of api:tick-hooks iterates it: one device
// mapping per seat, which is what a shared screen means in practice.
func (m *Match[S, A, O]) LocalSeats() []Seat {
	out := make([]Seat, 0, len(m.seats))
	for _, seat := range m.seats {
		if seat.LocalHuman() {
			out = append(out, seat)
		}
	}
	return out
}

// Submit queues one action for a slot — the intake half of
// api:tick-hooks. Safe for concurrent use, and safe to call every frame:
// under the newest-input intake policy a duplicate simply supersedes.
func (m *Match[S, A, O]) Submit(slot session.SlotID, action A) error {
	inbox, ok := m.inboxes[slot]
	if !ok {
		return fmt.Errorf("%w: %d", ErrUnknownSlot, slot)
	}
	inbox.Submit(action)
	return nil
}

// Inbox exposes a slot's mailbox, for a caller wiring its own transport
// or driving a scripted schedule.
func (m *Match[S, A, O]) Inbox(slot session.SlotID) (*session.Inbox[A], error) {
	inbox, ok := m.inboxes[slot]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownSlot, slot)
	}
	return inbox, nil
}

// Run drives the match on the calling goroutine until it ends or ctx is
// cancelled. A headless process uses this.
func (m *Match[S, A, O]) Run(ctx context.Context, tc session.TimeControl) error {
	for _, d := range m.drivers {
		d.ctx = ctx
	}
	err := m.sess.RunRealtime(ctx, tc)
	m.finish(err)
	return err
}

// Start drives the match on its own goroutine and returns immediately —
// what a client does, because Ebitengine insists on the main one. Wait
// for Done and read Err.
func (m *Match[S, A, O]) Start(ctx context.Context, tc session.TimeControl) {
	for _, d := range m.drivers {
		d.ctx = ctx
	}
	go func() {
		m.finish(m.sess.RunRealtime(ctx, tc))
	}()
}

// finish records the outcome once and closes Done.
func (m *Match[S, A, O]) finish(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-m.done:
		return
	default:
	}
	m.err = err
	close(m.done)
}

// Done is closed when the match has ended, however it ended.
func (m *Match[S, A, O]) Done() <-chan struct{} { return m.done }

// Over reports whether the match has ended, without blocking.
func (m *Match[S, A, O]) Over() bool {
	select {
	case <-m.done:
		return true
	default:
		return false
	}
}

// Err reports why the match ended. Nil after a normal end.
func (m *Match[S, A, O]) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.err
}

// Tick reports how many ticks have committed.
func (m *Match[S, A, O]) Tick() session.Tick { return m.sess.Tick() }
