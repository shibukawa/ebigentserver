package game_test

import (
	"context"
	"testing"

	"github.com/shibukawa/ebigentserver/examples/solo/game"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// play runs one unattended match and reports its final checkpoint and
// outcomes. It goes through api:roster and run.Match rather than building
// a session by hand, so the wrapper is on the path every test takes.
func play(t *testing.T, seed uint64) (session.Checkpoint, []session.SlotOutcome) {
	t.Helper()
	roster, err := run.NewRoster[game.State, game.Action, game.Observation](game.Options(), game.Slots())
	if err != nil {
		t.Fatal(err)
	}
	if err := roster.FillBots(game.NewAgent); err != nil {
		t.Fatal(err)
	}
	cfg := game.Config("test", seed)
	tap := &checkpointTap{}
	watch := run.Watch[game.State, game.Action, game.Observation](tap)
	cfg.Recorder = watch

	match, err := roster.Finalize(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := match.Run(context.Background(), session.Unlimited); err != nil {
		t.Fatal(err)
	}
	return tap.last, watch.Outcomes()
}

// TestMatchIsDeterministic is the claim everything else rests on: the
// same seed produces the same episode, bit for bit. The digest is pinned
// rather than only compared to itself, so a change to the physics or to
// the sample agents has to be acknowledged here rather than passing
// quietly (data:state-checkpoint).
func TestMatchIsDeterministic(t *testing.T) {
	const seed = 1
	first, _ := play(t, seed)
	second, _ := play(t, seed)

	if first != second {
		t.Fatalf("same seed diverged: %+v vs %+v", first, second)
	}
	const (
		wantTick   = session.Tick(180)
		wantWorld  = uint64(0x6531c96ad8360b7c)
		wantAction = uint64(0x5e25d09ef77fea5d)
	)
	if first.Tick != wantTick || first.WorldHash != wantWorld || first.ActionHash != wantAction {
		t.Errorf("pinned checkpoint moved: tick %d world %#016x action %#016x\n"+
			"if the rules or the sample agents changed on purpose, repin these",
			first.Tick, first.WorldHash, first.ActionHash)
	}
}

// TestCorpusCarriesBothOutcomes guards the thing that makes the corpus
// worth distilling. A run where the quarry always escapes, or always
// dies, teaches an enemy nothing; the sample is balanced so that neither
// happens, and a retune that breaks the balance should fail here rather
// than quietly produce a useless corpus.
func TestCorpusCarriesBothOutcomes(t *testing.T) {
	wins, losses := 0, 0
	for seed := uint64(1); seed <= 12; seed++ {
		_, outcomes := play(t, seed)
		for _, o := range outcomes {
			if o.Slot != game.Player {
				continue
			}
			switch o.Signal.Terminal {
			case session.Win:
				wins++
			case session.Lose:
				losses++
			default:
				t.Fatalf("seed %d: player ended %v, which is neither", seed, o.Signal.Terminal)
			}
		}
	}
	if wins == 0 || losses == 0 {
		t.Errorf("12 matches produced %d escapes and %d captures; a corpus needs both", wins, losses)
	}
}

// TestEnemiesDisagree checks that the two pursuit kinds are actually two.
// Distilling a mixed corpus of identical behaviors would produce one
// policy and hide that the kinds never differed.
func TestEnemiesDisagree(t *testing.T) {
	g := game.RuleSet{}
	state := g.Start(3)
	_, chaser := game.NewAgent(game.Enemy1)
	_, flanker := game.NewAgent(game.Enemy2)

	differed := false
	for tick := 0; tick < 60 && !state.Over; tick++ {
		chaser.Observe(g.Project(&state, game.Enemy1))
		flanker.Observe(g.Project(&state, game.Enemy2))
		a, okA := chaser.Decide(context.Background())
		b, okB := flanker.Decide(context.Background())
		if !okA || !okB {
			t.Fatalf("tick %d: an enemy declined to decide", tick)
		}
		if a.Move != b.Move {
			differed = true
		}
		g.Apply(&state, game.Enemy1, a)
		g.Apply(&state, game.Enemy2, b)
		g.Advance(&state)
	}
	if !differed {
		t.Error("chaser and flanker chose the same direction for 60 ticks; they are one kind, not two")
	}
}

// TestValidatorRejectsUndefinedDirection keeps api:action-validator
// occupied by a rule that can actually fail. A client sending this is
// broken or lying, and either way the session counts the rejection
// instead of applying it.
func TestValidatorRejectsUndefinedDirection(t *testing.T) {
	state := game.RuleSet{}.Start(1)
	v := game.Validator{}
	if err := v.Legal(&state, game.Player, game.Action{Move: game.Up}); err != nil {
		t.Fatalf("a legal move was rejected: %v", err)
	}
	if err := v.Legal(&state, game.Player, game.Action{Move: 200}); err == nil {
		t.Error("direction 200 was accepted")
	}
}

// checkpointTap keeps the last data:state-checkpoint. Everything else a
// recorder is asked for is discarded: this test cares only about whether
// two runs committed the same state.
type checkpointTap struct{ last session.Checkpoint }

var _ session.Recorder[game.State, game.Action, game.Observation] = (*checkpointTap)(nil)

func (c *checkpointTap) EpisodeStarted(session.EpisodeStart) {}
func (c *checkpointTap) Observed(session.Tick, session.SlotID, game.Observation, session.EvaluationSignal) {
}
func (c *checkpointTap) Decided(session.Tick, session.SlotID, game.Observation, game.Action, session.EvaluationSignal, int64) {
}
func (c *checkpointTap) Rejected(session.Tick, session.SlotID, string)        {}
func (c *checkpointTap) Lifecycle(session.Tick, session.State, session.State) {}
func (c *checkpointTap) WorldCommitted(session.Tick, *game.State)             {}
func (c *checkpointTap) Checkpointed(cp session.Checkpoint)                   { c.last = cp }
func (c *checkpointTap) Ended(session.Tick, []session.SlotOutcome)            {}
