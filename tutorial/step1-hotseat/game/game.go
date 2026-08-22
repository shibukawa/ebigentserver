// Package game holds the rules of tic-tac-toe and nothing else: no
// window, no mouse, no engine import.
//
// The split is not ceremony. Rules that do not import Ebitengine can be
// tested without opening a window, which is the difference between a
// test suite that runs in CI and one that does not. Later steps of this
// tutorial hand this same package to a session; step 1 only needs it to
// be honest about what it knows.
package game

// Mark is what occupies one cell.
type Mark uint8

const (
	// Empty is an unclaimed cell, and also "nobody" for Winner and Turn.
	Empty Mark = iota
	X
	O
)

// String renders a mark for the status line.
func (m Mark) String() string {
	switch m {
	case X:
		return "X"
	case O:
		return "O"
	default:
		return "-"
	}
}

// Other returns the mark that moves after m.
func (m Mark) Other() Mark {
	switch m {
	case X:
		return O
	case O:
		return X
	default:
		return Empty
	}
}

// Board is the 3x3 grid, row-major: 0 is top-left, 8 is bottom-right.
type Board [9]Mark

// Lines are the eight ways to win.
var Lines = [8][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // columns
	{0, 4, 8}, {2, 4, 6}, // diagonals
}

// State is one game in progress. The zero value is not playable; call
// New.
type State struct {
	// Board is the grid.
	Board Board
	// Turn is whose move it is, and Empty once the game is over.
	Turn Mark
	// Winner is Empty on a draw and on an unfinished game.
	Winner Mark
	// Line is the winning triple, meaningful only when Winner is set.
	Line [3]int
	// Over marks a finished game, won or drawn.
	Over bool
	// Moves counts marks placed.
	Moves int
}

// New deals an empty board with X to move.
func New() State { return State{Turn: X} }

// Legal reports whether the side to move may take cell.
func (s *State) Legal(cell int) bool {
	return !s.Over && cell >= 0 && cell < len(s.Board) && s.Board[cell] == Empty
}

// LegalMoves lists the cells the side to move may take, in board order.
// It is empty once the game is over.
func (s *State) LegalMoves() []int {
	var out []int
	for cell := range s.Board {
		if s.Legal(cell) {
			out = append(out, cell)
		}
	}
	return out
}

// Place takes cell for the side to move and reports whether it happened.
// An illegal cell leaves the state untouched, so a caller may hand it
// any click without checking first.
func (s *State) Place(cell int) bool {
	if !s.Legal(cell) {
		return false
	}
	mark := s.Turn
	s.Board[cell] = mark
	s.Moves++
	if line, won := s.Board.lineFor(mark); won {
		s.Winner, s.Line, s.Over, s.Turn = mark, line, true, Empty
		return true
	}
	if s.Moves == len(s.Board) {
		s.Over, s.Turn = true, Empty
		return true
	}
	s.Turn = mark.Other()
	return true
}

// Drawn reports a finished game with no winner.
func (s *State) Drawn() bool { return s.Over && s.Winner == Empty }

// lineFor returns the line m occupies, if any.
func (b Board) lineFor(m Mark) ([3]int, bool) {
	for _, line := range Lines {
		if b[line[0]] == m && b[line[1]] == m && b[line[2]] == m {
			return line, true
		}
	}
	return [3]int{}, false
}
