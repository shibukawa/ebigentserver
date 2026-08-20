package session

import "github.com/shibukawa/fixmath"

// Terminal is the end-of-episode judgement of one slot.
type Terminal uint8

const (
	// NotTerminal: the episode is still running for this slot.
	NotTerminal Terminal = iota
	// Win, Lose, Draw: normal game outcomes.
	Win
	Lose
	Draw
	// Abandoned: the episode stopped without a game outcome (operator
	// stop, controller gone, session aborted).
	Abandoned
)

func (t Terminal) String() string {
	switch t {
	case NotTerminal:
		return "not_terminal"
	case Win:
		return "win"
	case Lose:
		return "lose"
	case Draw:
		return "draw"
	case Abandoned:
		return "abandoned"
	default:
		return "invalid"
	}
}

// EvaluationSignal is data:evaluation-signal: the session-computed
// judgement of how a slot is doing, delivered with every observation and
// recorded for analysis. Without it an agent can act legally and still not
// know whether it is winning.
//
// The full field set exists from Phase 1 even though Phase 1 games fill
// only Terminal — plan.md's seam table: without these fields recorded per
// decision, Phase 7 cannot assign credit and the corpus is worthless.
// All numeric fields are fixed point (rule:no-float-in-simulation).
type EvaluationSignal struct {
	// Score is the game-visible score: points, resources, health.
	Score int64
	// Progress is distance to the win condition, normalized to [0, 1]
	// where the game can define it.
	Progress fixmath.F64
	// Evaluation is the heuristic value of the position — the game
	// equivalent of a chess eval.
	Evaluation fixmath.F64
	// RewardDelta is the change since the previous decision point, the
	// per-decision credit signal.
	RewardDelta fixmath.F64
	// Terminal is set only at the end of the episode.
	Terminal Terminal
}
