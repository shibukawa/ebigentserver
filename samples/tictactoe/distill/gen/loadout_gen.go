// Package gen: generated loadout "center-first" — a tactic selector over chips
// from the shared library (data:agent-loadout, concept:tactic-selector).
// Edits belong upstream in the loadout or the chips.
package gen

import (
	"context"

	"github.com/shibukawa/ebigentserver/samples/tictactoe/ttt"
	"github.com/shibukawa/ebigentserver/session"
)

// TacticDecide selects a tactic from the observation, then runs its chips.
func TacticDecide(obs ttt.Sight) (ttt.Move, bool) {
	switch {
	case obs.Board[4] == ttt.Empty:
		// tactic claim_center
		switch {
		case obs.Board[4] == ttt.Empty:
			// chip cell_4_empty→play_4 (coverage 52)
			return ttt.Move{Cell: 4}, true
		}
	case true:
		// fallback tactic leftmost
		switch {
		case obs.Board[0] == ttt.Empty:
			// chip cell_0_empty→play_0 (coverage 200)
			return ttt.Move{Cell: 0}, true
		case obs.Board[1] == ttt.Empty:
			// chip cell_1_empty→play_1 (coverage 176)
			return ttt.Move{Cell: 1}, true
		case obs.Board[2] == ttt.Empty:
			// chip cell_2_empty→play_2 (coverage 150)
			return ttt.Move{Cell: 2}, true
		case obs.Board[3] == ttt.Empty:
			// chip cell_3_empty→play_3 (coverage 66)
			return ttt.Move{Cell: 3}, true
		case obs.Board[5] == ttt.Empty:
			// chip cell_5_empty→play_5 (coverage 10)
			return ttt.Move{Cell: 5}, true
		case obs.Board[6] == ttt.Empty:
			// chip cell_6_empty→play_6 (coverage 20)
			return ttt.Move{Cell: 6}, true
		case obs.Board[7] == ttt.Empty:
			// chip cell_7_empty→play_7 (coverage 6)
			return ttt.Move{Cell: 7}, true
		case obs.Board[8] == ttt.Empty:
			// chip cell_8_empty→play_8 (coverage 6)
			return ttt.Move{Cell: 8}, true
		}
	}
	var zero ttt.Move
	return zero, false
}

// TacticAgent seats the loadout behind api:agent-interface.
type TacticAgent struct {
	last ttt.Sight
	has  bool
}

func (*TacticAgent) Joined(session.SlotID) {}

func (a *TacticAgent) Observe(obs ttt.Sight) { a.last, a.has = obs, true }

func (a *TacticAgent) Decide(context.Context) (ttt.Move, bool) {
	if !a.has {
		var zero ttt.Move
		return zero, false
	}
	return TacticDecide(a.last)
}

func (*TacticAgent) Ended(session.Result) {}
