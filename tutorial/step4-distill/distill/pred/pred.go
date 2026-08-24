// Package pred is the vocabulary layer of flow:behavior-tree-synthesis
// for tic-tac-toe: the judgements a chip is allowed to name.
//
// Step 4 exists because of what is in this file. The obvious vocabulary
// — "cell 4 is empty" and eight more like it — is a set of field reads,
// and a decision list built out of field reads cannot express "take the
// win if there is one". Mining against it produces rules with
// counterexamples, which are never approved, which leaves the generated
// agent silent in the situations it got wrong. It does not fail. It just
// comes out weaker than the thing it was distilled from.
//
// So the distinctions the raw fields do not express get names here, and
// the names are what a reviewer reads in a chip. That is the whole trick,
// and it is the same one samples/reversi plays with BestMoveIs: push the
// computation into a named term, and the decision list above it stays
// something a person can check.
//
// Two constraints hold by construction. Everything judges only what the
// sight carries, so rule:analysis-restricted-to-visible-fields cannot be
// violated by a predicate that compiles. And everything is integer
// comparison over a nine-cell board, so generated code that calls these
// stays within rule:generated-agent-code-is-deterministic and the
// per-tick cost bound.
package pred

import (
	"slices"

	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/game"
)

// WinningMoveIs reports that the observing seat can finish a line this
// turn, and that the cell to play is the given one.
//
// "The" cell, singular, matters. Two winning cells can be open at once,
// and a predicate that answered true for both would let two chips claim
// the same position — the decision list would take whichever came first
// and the recorded play would disagree with it half the time. So the tie
// is settled here, in the same order game.Lines is written, which is the
// order the hand-written bot settles it in too.
func WinningMoveIs(obs game.Sight, cell int) bool {
	c, ok := completes(obs, obs.Mark)
	return ok && c == cell
}

// BlockingMoveIs reports the same for the opponent's line: they finish
// next turn unless this cell is taken.
//
// It says nothing about whether blocking is the right move. A seat that
// can win outright should do that instead, and this predicate stays true
// in that position — the ordering is the decision list's job, not the
// predicate's. Keeping the two apart is what lets the miner discover the
// priority rather than being told it.
func BlockingMoveIs(obs game.Sight, cell int) bool {
	c, ok := completes(obs, rival(obs.Mark))
	return ok && c == cell
}

// PreferredCellIs reports that, with nothing urgent on the board, the
// cell the seat would take is this one: centre first, then corners, then
// edges, and only among cells the sight says are legal.
//
// This is taste rather than judgement, and it is the one predicate here
// that encodes a policy rather than a fact about the position. It earns
// its place by being the reason the mined list terminates: without it the
// quiet moves — most of the corpus — have nothing to be explained by.
func PreferredCellIs(obs game.Sight, cell int) bool {
	for _, c := range Preference {
		if slices.Contains(obs.Legal, c) {
			return c == cell
		}
	}
	return false
}

// Preference ranks the squares that neither win nor block. It is the
// same table game.Bot reads, kept here because a distilled agent must
// not depend on the bot it replaces.
var Preference = [9]int{4, 0, 2, 6, 8, 1, 3, 5, 7}

// completes returns the free cell that finishes a line for mark, in
// game.Lines order. It returns false when the seat is not to move: an
// empty Legal is the sight saying so, and a predicate that judged a
// position it was not asked about would put rules in the list for turns
// that never happened.
func completes(obs game.Sight, mark game.Mark) (int, bool) {
	if len(obs.Legal) == 0 || len(obs.Cells) != 9 {
		return 0, false
	}
	for _, line := range game.Lines {
		var empty int
		held, free := 0, 0
		for _, c := range line {
			switch obs.Cells[c] {
			case game.Empty:
				empty, free = int(c), free+1
			case mark:
				held++
			}
		}
		if held == 2 && free == 1 {
			return empty, true
		}
	}
	return 0, false
}

// rival is the other mark.
func rival(m game.Mark) game.Mark {
	if m == game.X {
		return game.O
	}
	return game.X
}
