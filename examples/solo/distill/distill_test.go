package distill_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/ebigentserver/examples/solo/distill"
	"github.com/shibukawa/ebigentserver/examples/solo/distill/gen/chaser"
	"github.com/shibukawa/ebigentserver/examples/solo/distill/gen/flanker"
	"github.com/shibukawa/ebigentserver/examples/solo/game"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// The committed generated code came from this corpus. Regenerating from
// anything else would compare two different things.
const (
	corpusMatches = 16
	corpusSeed    = 1
)

// corpus records the canonical corpus once per test binary.
func corpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := distill.Play(context.Background(), root, corpusMatches, corpusSeed); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestEveryDecisionIsCovered is the mining claim: the vocabulary is rich
// enough that no recorded decision is left unexplained. An uncovered
// decision means the enemy did something the predicates cannot describe,
// and Synthesize refuses rather than generating a policy with a hole in
// it.
func TestEveryDecisionIsCovered(t *testing.T) {
	root := corpus(t)
	for _, kind := range distill.Kinds() {
		c, err := distill.Compile(root, kind)
		if err != nil {
			t.Fatalf("%s: %v", kind.Name, err)
		}
		if len(c.Records) == 0 {
			t.Fatalf("%s: no decisions were recorded", kind.Name)
		}
		approved := c.Library.Approved()
		if len(approved) == 0 {
			t.Fatalf("%s: nothing was approved", kind.Name)
		}
		for _, chip := range approved {
			if chip.Counterexamples != 0 {
				t.Errorf("%s: chip %s was approved with %d counterexamples",
					kind.Name, chip.Key(), chip.Counterexamples)
			}
			if len(chip.Evidence) == 0 {
				t.Errorf("%s: chip %s carries no evidence, so nobody can review it",
					kind.Name, chip.Key())
			}
		}
	}
}

// TestKindsDistillDifferently is what stops the vocabulary from being the
// policy in disguise. Both enemies are mined with the same predicates
// over the same corpus; if the resulting decision lists were equal, the
// predicates would be doing the deciding.
func TestKindsDistillDifferently(t *testing.T) {
	root := corpus(t)
	sets := map[string]map[string]string{}
	for _, kind := range distill.Kinds() {
		c, err := distill.Compile(root, kind)
		if err != nil {
			t.Fatal(err)
		}
		rules := map[string]string{}
		for _, chip := range c.Library.Approved() {
			rules[chip.Condition] = chip.Action
		}
		sets[kind.Name] = rules
	}
	a, b := sets[game.KindChaser], sets[game.KindFlanker]
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("a kind produced no rules")
	}
	same := len(a) == len(b)
	if same {
		for cond, act := range a {
			if b[cond] != act {
				same = false
				break
			}
		}
	}
	if same {
		t.Error("both enemy kinds distilled to the same decision list; the vocabulary is deciding, not the corpus")
	}
}

// TestGeneratedCodeMatchesTheCorpus keeps the committed source honest: it
// is what this corpus produces today, not what it produced once. A change
// to the rules, the sample enemies, or the vocabulary fails here until
// solo-distill is run again and the diff is reviewed.
func TestGeneratedCodeMatchesTheCorpus(t *testing.T) {
	root := corpus(t)
	for _, kind := range distill.Kinds() {
		c, err := distill.Compile(root, kind)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("gen", kind.Package, "agent_gen.go")
		committed, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !bytes.Equal(committed, c.Agent) {
			t.Errorf("%s is stale; rerun: go run ./examples/solo/cmd/solo-distill", path)
		}
	}
}

// TestCompiledPolicyMatchesTheOriginal is the equivalence claim, checked
// against every situation the corpus actually holds: on each recorded
// observation, the compiled decision list chooses what the hand-written
// enemy chose.
func TestCompiledPolicyMatchesTheOriginal(t *testing.T) {
	root := corpus(t)
	compiled := map[string]func(game.Observation) (game.Action, bool){
		game.KindChaser:  chaser.Decide,
		game.KindFlanker: flanker.Decide,
	}
	for _, kind := range distill.Kinds() {
		records, err := distill.Records(root, kind.Slot)
		if err != nil {
			t.Fatal(err)
		}
		_, original := game.NewAgent(kind.Slot)
		decide := compiled[kind.Name]

		undecided := 0
		for _, rec := range records {
			var obs game.Observation
			if err := json.Unmarshal(rec.Obs, &obs); err != nil {
				t.Fatalf("%s: recorded observation is not decodable: %v", kind.Name, err)
			}
			original.Observe(obs)
			want, ok := original.Decide(context.Background())
			if !ok {
				t.Fatalf("%s: the original declined to decide at %s tick %d",
					kind.Name, rec.Episode, rec.Tick)
			}
			got, ok := decide(obs)
			if !ok {
				undecided++
				continue
			}
			if got != want {
				t.Fatalf("%s at %s tick %d: compiled chose %v, original chose %v",
					kind.Name, rec.Episode, rec.Tick, got.Move, want.Move)
			}
		}
		if undecided != 0 {
			t.Errorf("%s: the compiled list had no rule for %d of %d recorded situations",
				kind.Name, undecided, len(records))
		}
	}
}

// TestDistilledEnemiesPlayTheSameMatch is the strongest form of the
// claim and the reason the loop is worth closing: seat the generated
// enemies instead of the hand-written ones and the whole episode comes
// out identical, tick for tick, down to the data:state-checkpoint.
//
// A policy that agrees on recorded situations could still diverge the
// moment it reaches an unrecorded one, and a divergence compounds. This
// is the test that would catch it.
func TestDistilledEnemiesPlayTheSameMatch(t *testing.T) {
	for seed := uint64(1); seed <= 4; seed++ {
		original := playWith(t, seed, game.NewAgent)
		generated := playWith(t, seed, func(slot session.SlotID) (string, session.Agent[game.Observation, game.Action]) {
			switch slot {
			case game.Enemy1:
				return game.KindChaser, &chaser.Chaser{}
			case game.Enemy2:
				return game.KindFlanker, &flanker.Flanker{}
			default:
				return game.NewAgent(slot)
			}
		})
		if original != generated {
			t.Fatalf("seed %d: the distilled enemies played a different match\n original %+v\ngenerated %+v",
				seed, original, generated)
		}
	}
}

// playWith runs one unattended match and returns its last checkpoint.
func playWith(t *testing.T, seed uint64, newAgent func(session.SlotID) (string, session.Agent[game.Observation, game.Action])) session.Checkpoint {
	t.Helper()
	roster, err := run.NewRoster[game.State, game.Action, game.Observation](game.Options(), game.Slots())
	if err != nil {
		t.Fatal(err)
	}
	if err := roster.FillBots(newAgent); err != nil {
		t.Fatal(err)
	}
	cfg := game.Config("equivalence", seed)
	tap := &checkpointTap{}
	cfg.Recorder = tap
	match, err := roster.Finalize(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := match.Run(context.Background(), session.Unlimited); err != nil {
		t.Fatal(err)
	}
	return tap.last
}

type checkpointTap struct{ last session.Checkpoint }

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
