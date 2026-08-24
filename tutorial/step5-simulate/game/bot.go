package game

import (
	"context"
	"slices"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/msg"
)

// Bot is the stand-in for the seat nobody took: take the win if there is
// one, block the loss if there is one, otherwise take the most useful
// free square.
//
// It is a decision list — four rules read in order — rather than a
// search, and both halves of that are deliberate.
//
// A search would play tic-tac-toe perfectly, and a perfect opponent
// records a corpus of nothing but draws and losses. A recording that
// never contains a win cannot teach what winning looks like, so the
// stand-in has to be beatable to be worth playing (TestTheBotCanBeBeaten
// fixes one line that beats it).
//
// And a decision list is the shape step 4 mines back out of the log. The
// interesting comparison is not whether a distilled agent is strong; it
// is whether the recording alone was enough to recover a policy somebody
// wrote by hand. That comparison needs the hand-written one to exist
// first, which is this file.
type Bot struct {
	sight Sight
}

var _ session.Agent[Sight, msg.Move] = (*Bot)(nil)

// Joined records nothing. Which seat this is, and which mark it plays,
// arrive on every sight — so the bot has one source of truth rather than
// two that can disagree.
func (*Bot) Joined(session.SlotID) {}

// Observe retains the latest sight. Nothing is decided here: the session
// delivers to every seat before any of them acts, and work done in this
// call would sit on the tick.
func (b *Bot) Observe(sight Sight) { b.sight = sight }

// Decide picks a cell from the last sight, and only from the last sight.
// It never sees msg.TTTWorld — for tic-tac-toe the two carry the same
// board, but the seam is what makes this same bot survive a game where
// they do not.
func (b *Bot) Decide(context.Context) (msg.Move, bool) {
	s := b.sight
	if len(s.Legal) == 0 {
		// Not this seat's turn, or the game is over. The sight says so,
		// so the bot needs no copy of the rules to know it — and a
		// controller that answered anyway would have every answer
		// refused by the validator and written to the events stream as
		// a rejection.
		return msg.Move{}, false
	}
	if cell, ok := completes(s.Cells, s.Mark); ok {
		return msg.Move{Cell: cell}, true
	}
	if cell, ok := completes(s.Cells, rival(s.Mark)); ok {
		return msg.Move{Cell: cell}, true
	}
	for _, cell := range preference {
		if slices.Contains(s.Legal, int(cell)) {
			return msg.Move{Cell: cell}, true
		}
	}
	// Unreachable while the sight is honest, since a non-empty Legal
	// holds one of the nine. Taking the first legal cell rather than
	// panicking keeps a bad sight from ending the match.
	return msg.Move{Cell: uint8(s.Legal[0])}, true
}

// Ended does nothing. A bot that learned between matches would put the
// order they were played in into the corpus, and nothing downstream
// reads a corpus in order.
func (*Bot) Ended(session.Result) {}

// preference ranks the squares that neither win nor block: centre, then
// corners, then edges. It is the whole of the bot's taste, and it is a
// table rather than a rule because that is how a decision list ends.
var preference = [9]uint8{4, 0, 2, 6, 8, 1, 3, 5, 7}

// completes reports a free cell that finishes a line for mark — the win
// when mark is the bot's own, the block when it is the opponent's.
func completes(cells []Mark, mark Mark) (uint8, bool) {
	for _, line := range Lines {
		if cell, ok := gap(cells, line, mark); ok {
			return cell, true
		}
	}
	return 0, false
}

// gap reports the one empty cell of a line whose other two hold mark.
func gap(cells []Mark, line [3]uint8, mark Mark) (uint8, bool) {
	var empty uint8
	held, free := 0, 0
	for _, cell := range line {
		switch cells[cell] {
		case Empty:
			empty, free = cell, free+1
		case mark:
			held++
		}
	}
	return empty, held == 2 && free == 1
}

// rival is the other mark.
func rival(m Mark) Mark {
	if m == X {
		return O
	}
	return X
}
