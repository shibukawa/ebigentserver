package ttt_test

import (
	"context"
	"testing"

	"github.com/shibukawa/ebigentserver/samples/tictactoe/ttt"
	"github.com/shibukawa/ebigentserver/session"
)

func newGame(t *testing.T, x, o session.Agent[ttt.Observation, ttt.Move]) *session.Session[ttt.State, ttt.Move, ttt.Observation] {
	t.Helper()
	s, err := session.New(session.Config[ttt.State, ttt.Move, ttt.Observation]{
		ID:        "ttt-test",
		Slots:     ttt.Slots(),
		RuleSet:   ttt.RuleSet{},
		Validator: ttt.Validator{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	if err := s.Admit(ttt.SlotX, x); err != nil {
		t.Fatal(err)
	}
	if err := s.Admit(ttt.SlotO, o); err != nil {
		t.Fatal(err)
	}
	return s
}

// Phase 1 completion criterion: a scripted "human" and a bot sit at the
// same api:agent-interface and one game finishes.
func TestScriptedHumanBeatsBot(t *testing.T) {
	human := &ttt.Script{Moves: []ttt.Move{{Cell: 0}, {Cell: 4}, {Cell: 8}}}
	bot := &ttt.Bot{}
	s := newGame(t, human, bot)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	// X plays 0, 4, 8; the first-empty bot answers 1 and 2; the 0-4-8
	// diagonal wins on move 5.
	if len(human.Results) != 1 || human.Results[0].Signal.Terminal != session.Win {
		t.Fatalf("human result = %+v, want one win", human.Results)
	}
	if human.Slot() != ttt.SlotX {
		t.Fatalf("human seated at slot %d, want %d", human.Slot(), ttt.SlotX)
	}
}

// decision:no-ai-game-mode: swapping which kind of controller occupies a
// slot is pure session configuration — the same game hosts bot vs bot
// unattended.
func TestBotVsBotCompletes(t *testing.T) {
	x, o := &ttt.Bot{}, &ttt.Bot{}
	s := newGame(t, x, o)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	// First-empty vs first-empty: X takes 0,2,4,6 and wins the 2-4-6
	// anti-diagonal on tick 7.
	if got := s.Tick(); got != 7 {
		t.Fatalf("game length = %d ticks, want 7", got)
	}
}

func TestBotVsBotIsDeterministic(t *testing.T) {
	final := func() session.Tick {
		s := newGame(t, &ttt.Bot{}, &ttt.Bot{})
		if err := s.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		return s.Tick()
	}
	if a, b := final(), final(); a != b {
		t.Fatalf("two identical matches diverged: %d vs %d ticks", a, b)
	}
}

func TestIllegalMovesAreRejectedWithoutStateChange(t *testing.T) {
	v := ttt.Validator{}
	s := ttt.RuleSet{}.Start(0)
	ttt.RuleSet{}.Apply(&s, ttt.SlotX, ttt.Move{Cell: 4})

	if err := v.Legal(&s, ttt.SlotO, ttt.Move{Cell: 4}); err == nil {
		t.Error("occupied cell must be illegal")
	}
	if err := v.Legal(&s, ttt.SlotX, ttt.Move{Cell: 0}); err == nil {
		t.Error("out-of-turn move must be illegal")
	}
	if err := v.Legal(&s, ttt.SlotO, ttt.Move{Cell: 9}); err == nil {
		t.Error("out-of-range cell must be illegal")
	}
	if err := v.Legal(&s, ttt.SlotO, ttt.Move{Cell: 0}); err != nil {
		t.Errorf("legal move rejected: %v", err)
	}
}

// A script that feeds only an illegal move exhausts its list on the retry
// and the session drains with the unfinished game abandoned — the state
// the illegal move aimed at was never touched.
func TestIllegalMoveNeverReachesTheBoard(t *testing.T) {
	x := &ttt.Script{Moves: []ttt.Move{{Cell: 4}}}
	o := &ttt.Script{Moves: []ttt.Move{{Cell: 4}}} // occupied: illegal
	s := newGame(t, x, o)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	if o.Results[0].Signal.Terminal != session.Abandoned {
		t.Fatalf("O result = %v, want abandoned", o.Results[0].Signal.Terminal)
	}
}

func TestDrawGame(t *testing.T) {
	// 0 1 2 / 3 4 5 / 6 7 8 filled X O X / X O O / O X X is a draw:
	// X: 0,2,3,7,8  O: 1,4,5,6
	x := &ttt.Script{Moves: []ttt.Move{{Cell: 0}, {Cell: 2}, {Cell: 3}, {Cell: 7}, {Cell: 8}}}
	o := &ttt.Script{Moves: []ttt.Move{{Cell: 1}, {Cell: 4}, {Cell: 5}, {Cell: 6}}}
	s := newGame(t, x, o)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if x.Results[0].Signal.Terminal != session.Draw || o.Results[0].Signal.Terminal != session.Draw {
		t.Fatalf("results = %v / %v, want draw / draw",
			x.Results[0].Signal.Terminal, o.Results[0].Signal.Terminal)
	}
}
