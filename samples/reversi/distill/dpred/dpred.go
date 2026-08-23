// Package dpred is reversi's derived-predicate runtime: the vocabulary
// layer of flow:behavior-tree-synthesis realized as ordinary, reviewable
// Go. Where tic-tac-toe's predicates were raw field reads
// ("obs.Board[4] == ttt.Empty"), reversi's are judgements
// (data:derived-predicate with `body: generated Go code` — here written
// and reviewed by hand, referenced by the generator): "the greedy best
// move is cell 19" compresses the whole argmax-over-affordances
// computation into one named term a developer can read in a chip and a
// generated decision list can call.
//
// Everything here judges only Sight fields the slot can see
// (rule:analysis-restricted-to-visible-fields holds by construction),
// and uses only integer math, so generated agents that call these stay
// deterministic (rule:generated-agent-code-is-deterministic).
package dpred

import "github.com/shibukawa/ebigentserver/samples/reversi/reversi"

// BestMoveIs reports whether the greedy choice over Sight.Legal —
// maximum Flips, ties broken by the first (lowest-cell) entry, exactly
// GreedyBot's argmax — is a placement on the given cell. False when the
// sight carries no legal moves (not this slot's turn) or when the
// only legal move is the forced pass.
func BestMoveIs(obs reversi.Sight, cell uint8) bool {
	if len(obs.Legal) == 0 || obs.Legal[0].Move.Pass {
		return false
	}
	best := obs.Legal[0]
	for _, lm := range obs.Legal[1:] {
		if lm.Flips > best.Flips {
			best = lm
		}
	}
	return best.Move.Cell == cell
}

// MustPass reports whether the observing slot is to move with no legal
// placement: Sight.Legal is the single forced-pass entry (the
// only position a pass entry can occupy, per reversi.LegalMoves).
func MustPass(obs reversi.Sight) bool {
	return len(obs.Legal) > 0 && obs.Legal[0].Move.Pass
}
