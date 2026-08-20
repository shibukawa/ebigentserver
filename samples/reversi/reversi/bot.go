package reversi

import (
	"context"

	"github.com/shibukawa/ebigentserver/session"
)

// Both bots decide purely from Observation.Legal — neither carries a rule
// engine, which is the point of putting legal enumeration in the
// observation. They stay deliberately minimal: the project's AI depth is
// meant to come from distilling recorded episodes into behavior trees
// (Phase 7), not from hand-written search.

// FirstBot plays the first legal move: the weakest deterministic policy,
// tic-tac-toe's bot transplanted.
type FirstBot struct {
	last Observation
}

var _ session.Agent[Observation, Move] = (*FirstBot)(nil)

// Joined does nothing.
func (*FirstBot) Joined(session.SlotID) {}

// Observe retains the latest observation.
func (b *FirstBot) Observe(obs Observation) { b.last = obs }

// Decide plays Legal[0].
func (b *FirstBot) Decide(context.Context) (Move, bool) {
	if len(b.last.Legal) == 0 {
		return Move{}, false
	}
	return b.last.Legal[0].Move, true
}

// Ended does nothing.
func (*FirstBot) Ended(session.Result) {}

// GreedyBot is the 1-ply "search" controller: it maximizes immediate
// flips using the affordance the observation carries, ties broken by
// lowest cell. Meaningfully different from FirstBot, which is what
// sample:reversi's larger action space exists to show.
type GreedyBot struct {
	last Observation
}

var _ session.Agent[Observation, Move] = (*GreedyBot)(nil)

// Joined does nothing.
func (*GreedyBot) Joined(session.SlotID) {}

// Observe retains the latest observation.
func (b *GreedyBot) Observe(obs Observation) { b.last = obs }

// Decide plays the legal move with the most flips; Legal's stable cell
// order makes the tie-break deterministic.
func (b *GreedyBot) Decide(context.Context) (Move, bool) {
	if len(b.last.Legal) == 0 {
		return Move{}, false
	}
	best := b.last.Legal[0]
	for _, lm := range b.last.Legal[1:] {
		if lm.Flips > best.Flips {
			best = lm
		}
	}
	return best.Move, true
}

// Ended does nothing.
func (*GreedyBot) Ended(session.Result) {}
