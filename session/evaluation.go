package session

import (
	"encoding/json"
	"fmt"

	"github.com/shibukawa/fixmath"
)

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

// MarshalText writes the outcome as its name.
//
// A game that delivers the signal inside its concept:sight has this
// recorded in every decision row, and a bare 3 in that column is
// something a reader has to look up while a "draw" is not. The outcomes
// stream next to it has always spelled the same field out
// (episode.Outcome.Result), so this is the sight catching up rather than
// a new convention.
func (t Terminal) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

// UnmarshalText reads a name back.
func (t *Terminal) UnmarshalText(text []byte) error {
	for _, known := range terminals {
		if known.String() == string(text) {
			*t = known
			return nil
		}
	}
	return fmt.Errorf("session: %q is not a terminal outcome", text)
}

// UnmarshalJSON reads either spelling: the name MarshalText writes, or
// the bare number this field used to be recorded as.
//
// Both, because a corpus outlives the code that wrote it. Episodes
// recorded before the name existed are still on disk and still valid
// evidence — a reader that could not open them would make the change to
// the format a change to the data (policy:episode-data-governance), and
// there is nothing wrong with the data.
func (t *Terminal) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var name string
		if err := json.Unmarshal(data, &name); err != nil {
			return err
		}
		return t.UnmarshalText([]byte(name))
	}
	var n uint8
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("session: terminal outcome: %w", err)
	}
	if int(n) >= len(terminals) {
		return fmt.Errorf("session: %d is not a terminal outcome", n)
	}
	*t = Terminal(n)
	return nil
}

// terminals is the value set, in declaration order, so the two decoders
// above and String below cannot drift apart.
var terminals = [...]Terminal{NotTerminal, Win, Lose, Draw, Abandoned}

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
// judgement of how a slot is doing, delivered with every sight and
// recorded for analysis. Without it an agent can act legally and still not
// know whether it is winning.
//
// The full field set exists from Phase 1 even though Phase 1 games fill
// only Terminal — plan.md's seam table: without these fields recorded per
// decision, Phase 7 cannot assign credit and the corpus is worthless.
// All numeric fields are fixed point (rule:no-float-in-simulation).
//
// The json tags are here because a game that delivers the signal inside
// its concept:sight — the arrangement this framework recommends — has it
// recorded verbatim in the decisions stream. Those names are therefore
// column names, and they are spelled the way the rest of
// data:episode-log spells its columns rather than the way Go spells its
// fields. Fixed-point values are written as their raw int64 bits, as
// they are everywhere else in the log.
type EvaluationSignal struct {
	// Score is the game-visible score: points, resources, health.
	Score int64 `json:"score"`
	// Progress is distance to the win condition, normalized to [0, 1]
	// where the game can define it.
	Progress fixmath.F64 `json:"progress"`
	// Evaluation is the heuristic value of the position — the game
	// equivalent of a chess eval.
	Evaluation fixmath.F64 `json:"evaluation"`
	// RewardDelta is the change since the previous decision point, the
	// per-decision credit signal.
	RewardDelta fixmath.F64 `json:"reward_delta"`
	// Terminal is set only at the end of the episode.
	Terminal Terminal `json:"terminal"`
}
