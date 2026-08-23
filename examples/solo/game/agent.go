package game

import (
	"context"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// The enemy kinds. A kind is a label on the seat, which is what puts it
// in the data:episode-log header and lets a corpus be separated per kind
// later — distilling one behavior out of three mixed pursuers would
// produce a policy none of them had.
const (
	KindRunner  = "runner"
	KindChaser  = "chaser"
	KindFlanker = "flanker"
)

// NewAgent supplies the controller for a seat nobody is sitting in. Both
// entry points use it: the client fills the enemy seats with it after the
// player takes theirs, and the headless run fills every seat including
// the player's, which is how the same rules produce a corpus with nobody
// watching.
//
// The kinds alternate by slot so a single match records more than one
// pursuit style. Deliberately minimal play: this project's AI depth is
// meant to come from distilling these episodes, not from hand-written
// cleverness.
func NewAgent(slot session.SlotID) (string, session.Agent[Observation, Action]) {
	switch {
	case slot == Player:
		return KindRunner, &Runner{}
	case slot%2 == 0:
		return KindChaser, &Chaser{}
	default:
		return KindFlanker, &Flanker{}
	}
}

// Chaser closes whichever axis it is furthest off on — the blunt pursuit,
// and the one whose weakness a player learns first.
type Chaser struct {
	last Observation
	// Result is the outcome the session delivers at the end, kept so an
	// entry point can report it.
	Result session.Result
}

var _ session.Agent[Observation, Action] = (*Chaser)(nil)

// Guest does nothing; the observation carries the slot.
func (*Chaser) Guest(session.SlotID) {}

// Observe retains the latest observation.
func (c *Chaser) Observe(obs Observation) { c.last = obs }

// Decide moves along the axis with the larger remaining gap.
func (c *Chaser) Decide(context.Context) (Action, bool) {
	dx, dy := gap(c.last.Self, c.last.Quarry)
	if dx.Abs() >= dy.Abs() {
		return Action{Move: horizontal(dx)}, true
	}
	return Action{Move: vertical(dy)}, true
}

// Ended is the session-end callback.
func (c *Chaser) Ended(r session.Result) { c.Result = r }

// Flanker closes the smaller gap first, so it arrives across the player's
// path rather than behind them. Against the Runner below the two kinds
// fail in different situations, which is the whole reason for having two.
type Flanker struct {
	last   Observation
	Result session.Result
}

var _ session.Agent[Observation, Action] = (*Flanker)(nil)

// Guest does nothing.
func (*Flanker) Guest(session.SlotID) {}

// Observe retains the latest observation.
func (f *Flanker) Observe(obs Observation) { f.last = obs }

// Decide moves along the axis with the smaller remaining gap, until that
// axis is closed and only the other one is left.
func (f *Flanker) Decide(context.Context) (Action, bool) {
	dx, dy := gap(f.last.Self, f.last.Quarry)
	if dx.Abs() == 0 {
		return Action{Move: vertical(dy)}, true
	}
	if dy.Abs() == 0 || dx.Abs() <= dy.Abs() {
		return Action{Move: horizontal(dx)}, true
	}
	return Action{Move: vertical(dy)}, true
}

// Ended is the session-end callback.
func (f *Flanker) Ended(r session.Result) { f.Result = r }

// Runner is the player's stand-in for an unattended run: it backs away
// from the nearest enemy along whichever axis it has more field left on.
//
// It is not a good player, and that is useful. A corpus where the quarry
// sometimes gets cornered contains both outcomes, and a corpus of nothing
// but escapes would teach an enemy nothing.
type Runner struct {
	last   Observation
	Result session.Result
}

var _ session.Agent[Observation, Action] = (*Runner)(nil)

// Guest does nothing.
func (*Runner) Guest(session.SlotID) {}

// Observe retains the latest observation.
func (r *Runner) Observe(obs Observation) { r.last = obs }

// Decide tries each direction and keeps the one that leaves the nearest
// pursuer furthest away — one ply, no further.
//
// One ply is enough to make the episode interesting and little enough
// that nobody mistakes it for the point. Backing straight away from the
// closest enemy would be simpler still, and it walks into a wall almost
// every time, which produces a corpus of nothing but early captures.
func (r *Runner) Decide(context.Context) (Action, bool) {
	if len(r.last.Others) == 0 {
		return Action{Move: Stay}, true
	}
	best, bestRoom := Stay, fixmath.F64(0)
	for _, d := range []Dir{Stay, Up, Down, Left, Right} {
		cand := r.last.Self
		cand.Move = d
		step(&cand, playerSpeed)
		nearest := maxDist
		for _, other := range r.last.Others {
			nearest = nearest.Min(manhattan(cand, other))
		}
		// Ties keep the earlier direction, so the choice is a function
		// of the observation and nothing else.
		if d == Stay || nearest > bestRoom {
			best, bestRoom = d, nearest
		}
	}
	return Action{Move: best}, true
}

// Ended is the session-end callback.
func (r *Runner) Ended(res session.Result) { r.Result = res }

// gap is the vector from a to b: how far, and in which sign, b lies.
func gap(a, b Actor) (dx, dy fixmath.F64) {
	return b.X.Sub(a.X), b.Y.Sub(a.Y)
}

// horizontal turns a signed x gap into a direction.
func horizontal(dx fixmath.F64) Dir {
	if dx > 0 {
		return Right
	}
	if dx < 0 {
		return Left
	}
	return Stay
}

// vertical turns a signed y gap into a direction.
func vertical(dy fixmath.F64) Dir {
	if dy > 0 {
		return Down
	}
	if dy < 0 {
		return Up
	}
	return Stay
}

// room reports how much field is left in the direction of d from pos.
func room(pos, d, extent fixmath.F64) fixmath.F64 {
	if d > 0 {
		return extent.Sub(pos)
	}
	if d < 0 {
		return pos
	}
	return 0
}
