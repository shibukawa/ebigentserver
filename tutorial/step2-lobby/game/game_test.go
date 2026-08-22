package game_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step2-lobby/game"
)

// play applies a sequence of cells for whoever is to move.
func play(t *testing.T, cells ...uint8) game.State {
	t.Helper()
	var sim game.Simulation
	s := sim.Start(0)
	for i, cell := range cells {
		acting := sim.ActingSlots(&s)
		if len(acting) != 1 {
			t.Fatalf("move %d: %d seats acting, want 1", i, len(acting))
		}
		slot := acting[0]
		if err := (game.Validator{}).Legal(&s, slot, game.Action{Cell: cell}); err != nil {
			t.Fatalf("move %d: %v", i, err)
		}
		sim.Apply(&s, slot, game.Action{Cell: cell})
	}
	return s
}

func TestXMovesFirstAndTurnsAlternate(t *testing.T) {
	var sim game.Simulation
	s := sim.Start(0)
	if got := sim.ActingSlots(&s); len(got) != 1 || got[0] != game.SlotX {
		t.Fatalf("first acting seat = %v, want [X]", got)
	}
	sim.Apply(&s, game.SlotX, game.Action{Cell: 0})
	if got := sim.ActingSlots(&s); len(got) != 1 || got[0] != game.SlotO {
		t.Fatalf("second acting seat = %v, want [O]", got)
	}
}

func TestValidatorRefusesTheSeatThatIsNotToMove(t *testing.T) {
	var sim game.Simulation
	s := sim.Start(0)
	if err := (game.Validator{}).Legal(&s, game.SlotO, game.Action{Cell: 0}); err == nil {
		t.Fatal("O was allowed to move first")
	}
}

func TestOccupiedCellIsRefused(t *testing.T) {
	s := play(t, 4)
	if err := (game.Validator{}).Legal(&s, game.SlotO, game.Action{Cell: 4}); err == nil {
		t.Fatal("an occupied cell was accepted")
	}
}

func TestRowWinEndsTheGameAndNamesTheLine(t *testing.T) {
	s := play(t, 0, 3, 1, 4, 2) // X takes the top row
	if !s.Over {
		t.Fatal("game is not over")
	}
	if s.Winner != uint16(game.SlotX) {
		t.Fatalf("Winner = %d, want %d", s.Winner, game.SlotX)
	}
	if len(s.Line) != 3 || s.Line[0] != 0 || s.Line[2] != 2 {
		t.Fatalf("Line = %v, want the top row", s.Line)
	}
	if got := (game.Simulation{}).ActingSlots(&s); len(got) != 0 {
		t.Fatalf("ActingSlots = %v after the game ended, want none", got)
	}
}

func TestFullBoardWithoutALineIsADraw(t *testing.T) {
	s := play(t, 0, 1, 2, 4, 3, 5, 7, 6, 8)
	if !s.Over || s.Winner != 0 {
		t.Fatalf("Over=%v Winner=%d, want a draw", s.Over, s.Winner)
	}
	for _, slot := range game.Slots() {
		if got := (game.Simulation{}).Evaluate(&s, slot).Terminal; got != session.Draw {
			t.Fatalf("seat %d evaluates %v, want draw", slot, got)
		}
	}
}

// TestObservationCarriesTheLegalMoves is what lets a controller — the
// window now, a distilled agent later — choose without a rule engine of
// its own.
func TestObservationCarriesTheLegalMoves(t *testing.T) {
	s := play(t, 4)
	obs := (game.Simulation{}).Project(&s, game.SlotO)
	if obs.You != game.SlotO || obs.Mark != game.O {
		t.Fatalf("observation identifies %v as %v", obs.You, obs.Mark)
	}
	if len(obs.Legal) != 8 {
		t.Fatalf("Legal has %d cells, want 8", len(obs.Legal))
	}
	for _, cell := range obs.Legal {
		if cell == 4 {
			t.Fatal("the taken centre is listed as legal")
		}
	}
	// The seat that is not to move may take nothing.
	if idle := (game.Simulation{}).Project(&s, game.SlotX); len(idle.Legal) != 0 {
		t.Fatalf("the idle seat has %d legal moves, want 0", len(idle.Legal))
	}
}

// TestCodecRoundTripsTheBoard covers what step 1 never had to say: the
// board has a wire shape now, and both machines have to read it the same
// way.
func TestCodecRoundTripsTheBoard(t *testing.T) {
	s := play(t, 0, 3, 1, 4, 2)
	codec := game.Codec()

	var back game.State
	if err := codec.DecodeSnapshot(&back, codec.AppendSnapshot(nil, &s)); err != nil {
		t.Fatal(err)
	}
	if back.Winner != s.Winner || back.Moves != s.Moves || len(back.Cells) != len(s.Cells) {
		t.Fatalf("snapshot round trip lost something:\n got %+v\nwant %+v", back, s)
	}
	for i := range s.Cells {
		if back.Cells[i] != s.Cells[i] {
			t.Fatalf("cell %d = %d, want %d", i, back.Cells[i], s.Cells[i])
		}
	}

	// A delta from an earlier board has to reconstruct the later one.
	base := play(t, 0, 3)
	delta := codec.Diff(&base, &s)
	rebuilt := codec.Clone(&base)
	if err := codec.ApplyDelta(&rebuilt, delta); err != nil {
		t.Fatal(err)
	}
	for i := range s.Cells {
		if rebuilt.Cells[i] != s.Cells[i] {
			t.Fatalf("after the delta, cell %d = %d, want %d", i, rebuilt.Cells[i], s.Cells[i])
		}
	}
	if rebuilt.Winner != s.Winner {
		t.Fatalf("after the delta, Winner = %d, want %d", rebuilt.Winner, s.Winner)
	}
}

// TestCloneDoesNotAliasTheBoard is the trap a slice-holding state sets:
// a value copy would leave every retained baseline pointing at the
// newest board, and deltas would come out empty.
func TestCloneDoesNotAliasTheBoard(t *testing.T) {
	s := play(t, 4)
	clone := game.Codec().Clone(&s)
	(game.Simulation{}).Apply(&s, game.SlotO, game.Action{Cell: 0})
	if clone.Cells[0] != uint8(game.Empty) {
		t.Fatal("the clone shares its board with the live state")
	}
}
