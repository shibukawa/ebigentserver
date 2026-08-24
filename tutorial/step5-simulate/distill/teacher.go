package distill

import (
	"context"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/distill/pred"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/game"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/msg"
)

// Perfect is a minimax teacher: it searches the whole game and never
// loses. Tic-tac-toe is small enough that the search is exhaustive and
// exact, so this is the strongest tic-tac-toe player there is.
//
// It is here to answer the question step 4 left: what happens when the
// teacher is better than anything hand-written? The answer turns out not
// to be about strength at all, and the field below is why.
type Perfect struct {
	// Principled changes nothing about how well this plays and
	// everything about whether it can be distilled.
	//
	// Every opening move of tic-tac-toe draws under perfect play, so
	// the search returns the same value for all nine. Something has to
	// break the tie, and with Principled clear it is the order the
	// loop happens to run in — an implementation detail, invisible in
	// the position, and therefore invisible to any predicate. With it
	// set, ties go to the same centre-corner-edge preference the
	// hand-written bot used, which is a reason a rule can name.
	//
	// The strength is identical either way: every maximal move is
	// optimal, so choosing among them by taste costs nothing.
	Principled bool

	last game.Sight
}

var _ session.Agent[game.Sight, msg.Move] = (*Perfect)(nil)

// Joined records nothing.
func (*Perfect) Joined(session.SlotID) {}

// Observe retains the latest sight.
func (p *Perfect) Observe(obs game.Sight) { p.last = obs }

// Ended does nothing.
func (*Perfect) Ended(session.Result) {}

// Decide searches every continuation and takes a maximal move.
func (p *Perfect) Decide(context.Context) (msg.Move, bool) {
	if len(p.last.Legal) == 0 || len(p.last.Cells) != 9 {
		return msg.Move{}, false
	}
	board := make([]game.Mark, 9)
	copy(board, p.last.Cells)

	best, chosen := -2, p.last.Legal[0]
	for _, cell := range p.order() {
		board[cell] = p.last.Mark
		value := -negamax(board, other(p.last.Mark), -2, -best)
		board[cell] = game.Empty
		if value > best {
			best, chosen = value, cell
		}
	}
	return msg.Move{Cell: uint8(chosen)}, true
}

// order is the sequence the search considers moves in, which is also the
// tie-break: the first maximal move wins, so whichever order this
// returns decides what happens among equals.
func (p *Perfect) order() []int {
	if !p.Principled {
		return p.last.Legal
	}
	out := make([]int, 0, len(p.last.Legal))
	for _, want := range pred.Preference {
		for _, legal := range p.last.Legal {
			if legal == want {
				out = append(out, want)
			}
		}
	}
	return out
}

// negamax returns the value of the position for the side to move: +1 a
// forced win, 0 a draw, -1 a forced loss.
//
// The alpha-beta window is here because a search that did not prune
// would be a straw man. The cost this step measures against a decision
// list is the cost of a real search, pruned the way any of them are, and
// it is still four to five orders of magnitude — which is the point.
func negamax(board []game.Mark, turn game.Mark, alpha, beta int) int {
	if complete(board, other(turn)) {
		return -1
	}
	open := false
	for _, c := range board {
		if c == game.Empty {
			open = true
			break
		}
	}
	if !open {
		return 0
	}
	best := -2
	for i := range board {
		if board[i] != game.Empty {
			continue
		}
		board[i] = turn
		value := -negamax(board, other(turn), -beta, -max(alpha, best))
		board[i] = game.Empty
		if value > best {
			best = value
		}
		if best >= beta {
			// Nothing above can prefer this branch, so the rest of
			// it cannot change the answer.
			break
		}
	}
	return best
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// complete reports a finished line for mark.
func complete(board []game.Mark, mark game.Mark) bool {
	for _, line := range game.Lines {
		if board[line[0]] == mark && board[line[1]] == mark && board[line[2]] == mark {
			return true
		}
	}
	return false
}

func other(m game.Mark) game.Mark {
	if m == game.X {
		return game.O
	}
	return game.X
}
