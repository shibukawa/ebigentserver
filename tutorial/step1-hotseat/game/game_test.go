package game_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/tutorial/step1-hotseat/game"
)

// play applies a sequence of cells, failing on the first one the rules
// refuse. Tests read as move lists this way.
func play(t *testing.T, cells ...int) game.State {
	t.Helper()
	s := game.New()
	for i, cell := range cells {
		if !s.Place(cell) {
			t.Fatalf("move %d: Place(%d) refused in state %+v", i, cell, s)
		}
	}
	return s
}

func TestXMovesFirstAndTurnsAlternate(t *testing.T) {
	s := game.New()
	if s.Turn != game.X {
		t.Fatalf("Turn = %v, want X", s.Turn)
	}
	s.Place(0)
	if s.Turn != game.O {
		t.Fatalf("after X: Turn = %v, want O", s.Turn)
	}
	s.Place(1)
	if s.Turn != game.X {
		t.Fatalf("after O: Turn = %v, want X", s.Turn)
	}
}

func TestOccupiedCellIsRefusedAndChangesNothing(t *testing.T) {
	s := play(t, 4)
	before := s
	if s.Place(4) {
		t.Fatal("Place on an occupied cell reported success")
	}
	if s != before {
		t.Fatalf("refused move mutated the state:\n got %+v\nwant %+v", s, before)
	}
}

func TestOffBoardCellIsRefused(t *testing.T) {
	s := game.New()
	for _, cell := range []int{-1, 9, 100} {
		if s.Place(cell) {
			t.Errorf("Place(%d) reported success", cell)
		}
	}
}

func TestRowWinEndsTheGameAndNamesTheLine(t *testing.T) {
	// X: 0,1,2   O: 3,4
	s := play(t, 0, 3, 1, 4, 2)
	if !s.Over {
		t.Fatal("game is not over")
	}
	if s.Winner != game.X {
		t.Fatalf("Winner = %v, want X", s.Winner)
	}
	if s.Line != [3]int{0, 1, 2} {
		t.Fatalf("Line = %v, want [0 1 2]", s.Line)
	}
	if s.Turn != game.Empty {
		t.Fatalf("Turn = %v, want Empty once the game is over", s.Turn)
	}
}

func TestDiagonalWin(t *testing.T) {
	// X: 0,4,8   O: 1,2
	s := play(t, 0, 1, 4, 2, 8)
	if s.Winner != game.X || s.Line != [3]int{0, 4, 8} {
		t.Fatalf("Winner = %v, Line = %v; want X on [0 4 8]", s.Winner, s.Line)
	}
}

func TestOCanWin(t *testing.T) {
	// X: 0,1,8 (no line)   O: 3,4,5 (middle row)
	s := play(t, 0, 3, 1, 4, 8, 5)
	if s.Winner != game.O {
		t.Fatalf("Winner = %v, want O", s.Winner)
	}
	if s.Line != [3]int{3, 4, 5} {
		t.Fatalf("Line = %v, want [3 4 5]", s.Line)
	}
}

func TestFullBoardWithoutALineIsADraw(t *testing.T) {
	// X O X
	// X O O
	// O X X
	s := play(t, 0, 1, 2, 4, 3, 5, 7, 6, 8)
	if !s.Over {
		t.Fatal("game is not over")
	}
	if s.Winner != game.Empty {
		t.Fatalf("Winner = %v, want Empty", s.Winner)
	}
	if !s.Drawn() {
		t.Fatal("Drawn() = false, want true")
	}
	if s.Moves != 9 {
		t.Fatalf("Moves = %d, want 9", s.Moves)
	}
}

func TestNoMovesAfterTheGameEnds(t *testing.T) {
	s := play(t, 0, 3, 1, 4, 2) // X wins on the top row
	if s.Place(5) {
		t.Fatal("Place succeeded after the game ended")
	}
	if got := s.LegalMoves(); len(got) != 0 {
		t.Fatalf("LegalMoves() = %v, want none once the game is over", got)
	}
}

func TestLegalMovesShrinksByOnePerMove(t *testing.T) {
	s := game.New()
	for want := 9; want > 6; want-- {
		got := s.LegalMoves()
		if len(got) != want {
			t.Fatalf("LegalMoves() has %d cells, want %d", len(got), want)
		}
		s.Place(got[0])
	}
}
