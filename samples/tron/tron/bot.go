package tron

import (
	"context"

	"github.com/shibukawa/ebigentserver/samples/tron/msg"
	"github.com/shibukawa/ebigentserver/session"
)

// Bot is the minimal survivor: keep going until the next cell is blocked,
// then take the first open turn (right before left). Deterministic, no
// search — per the project's policy the interesting AI comes from
// distilling episodes, not from hand-writing survival heuristics.
type Bot struct {
	last Sight
}

var _ session.Agent[Sight, Input] = (*Bot)(nil)

// Joined does nothing.
func (*Bot) Joined(session.SlotID) {}

// Observe retains the latest sight.
func (b *Bot) Observe(obs Sight) { b.last = obs }

// Decide turns only when the current heading is doomed.
func (b *Bot) Decide(context.Context) (Input, bool) {
	s := &b.last.State
	p := player(s, b.last.You)
	if p == nil || !p.Alive || s.Over {
		return Input{}, false
	}
	if open(s, p.X, p.Y, p.Dir) {
		return Input{}, false // stay the course, no input this tick
	}
	for _, turn := range []uint8{(p.Dir + 1) % 4, (p.Dir + 3) % 4} {
		if open(s, p.X, p.Y, turn) {
			return Input{Tick: uint32(s.Tick), Dir: turn}, true
		}
	}
	return Input{}, false // boxed in; ride it out
}

// Ended does nothing.
func (*Bot) Ended(session.Result) {}

func open(s *State, x, y, dir uint8) bool {
	nx, ny, ok := step(x, y, dir)
	return ok && !blocked(s, nx, ny)
}

var _ = msg.TurnInput{}
