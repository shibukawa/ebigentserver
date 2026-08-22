package reversi_test

import (
	"context"
	"testing"

	"github.com/shibukawa/ebigentserver/samples/reversi/reversi"
	"github.com/shibukawa/ebigentserver/session"
)

func TestOpeningLegalMoves(t *testing.T) {
	g := reversi.Simulation{}
	s := g.Start(0)
	obs := g.Project(&s, reversi.SlotBlack)
	// Black's classical four openings: d3(19), c4(26), f5(37), e6(44),
	// each flipping exactly one disc, enumerated in cell order.
	want := []uint8{19, 26, 37, 44}
	if len(obs.Legal) != len(want) {
		t.Fatalf("legal moves = %+v, want cells %v", obs.Legal, want)
	}
	for i, lm := range obs.Legal {
		if lm.Move.Cell != want[i] || lm.Move.Pass || lm.Flips != 1 {
			t.Errorf("legal[%d] = %+v, want cell %d flips 1", i, lm, want[i])
		}
	}
	// White, not to move, gets no legal list.
	if got := g.Project(&s, reversi.SlotWhite).Legal; got != nil {
		t.Errorf("non-acting slot got legal moves: %+v", got)
	}
}

func TestValidator(t *testing.T) {
	v := reversi.Validator{}
	s := reversi.Simulation{}.Start(0)
	if err := v.Legal(&s, reversi.SlotBlack, reversi.Move{Cell: 19}); err != nil {
		t.Errorf("d3 rejected: %v", err)
	}
	if err := v.Legal(&s, reversi.SlotBlack, reversi.Move{Cell: 0}); err == nil {
		t.Error("non-flipping cell must be illegal")
	}
	if err := v.Legal(&s, reversi.SlotBlack, reversi.Move{Cell: 27}); err == nil {
		t.Error("occupied cell must be illegal")
	}
	if err := v.Legal(&s, reversi.SlotWhite, reversi.Move{Cell: 19}); err == nil {
		t.Error("out-of-turn move must be illegal")
	}
	if err := v.Legal(&s, reversi.SlotBlack, reversi.Move{Pass: true}); err == nil {
		t.Error("pass with legal moves must be illegal")
	}
}

func TestApplyFlips(t *testing.T) {
	g := reversi.Simulation{}
	s := g.Start(0)
	g.Apply(&s, reversi.SlotBlack, reversi.Move{Cell: 19}) // d3
	// d4 (27) flipped to black; d3 placed.
	if s.Board[19] != reversi.Black || s.Board[27] != reversi.Black {
		t.Fatalf("d3 play: board[19]=%v board[27]=%v, want black/black", s.Board[19], s.Board[27])
	}
	if s.Next != reversi.SlotWhite {
		t.Fatalf("next = %v, want white", s.Next)
	}
	sig := g.Evaluate(&s, reversi.SlotBlack)
	if sig.Score != 4 { // 2 + placed + flipped
		t.Fatalf("black score = %d, want 4", sig.Score)
	}
}

func newMatch(t *testing.T, black, white session.Agent[reversi.Observation, reversi.Move], cfg func(*session.Config[reversi.State, reversi.Move, reversi.Observation])) *session.Session[reversi.State, reversi.Move, reversi.Observation] {
	t.Helper()
	c := session.Config[reversi.State, reversi.Move, reversi.Observation]{
		ID:         "reversi-test",
		Slots:      reversi.Slots(),
		Simulation: reversi.Simulation{},
		Validator:  reversi.Validator{},
		Canonical:  reversi.Canonical,
		Clock:      func() int64 { return 0 }, // latency-free logs for byte comparisons
	}
	if cfg != nil {
		cfg(&c)
	}
	s, err := session.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	if err := s.Admit(reversi.SlotBlack, black); err != nil {
		t.Fatal(err)
	}
	if err := s.Admit(reversi.SlotWhite, white); err != nil {
		t.Fatal(err)
	}
	return s
}

// AI vs AI: two different controllers behind the same interface finish a
// full game (decision:samples-as-test-infrastructure's first real use).
func TestGreedyVsFirstCompletes(t *testing.T) {
	s := newMatch(t, &reversi.GreedyBot{}, &reversi.FirstBot{}, nil)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	if s.Tick() < 40 {
		t.Fatalf("game suspiciously short: %d ticks", s.Tick())
	}
}

func TestMatchIsDeterministic(t *testing.T) {
	run := func() session.Tick {
		s := newMatch(t, &reversi.GreedyBot{}, &reversi.FirstBot{}, nil)
		if err := s.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return s.Tick()
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("two identical matches diverged: %d vs %d ticks", a, b)
	}
}
