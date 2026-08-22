package distill_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/matchloop"
	"github.com/shibukawa/ebigentserver/samples/reversi/distill"
	"github.com/shibukawa/ebigentserver/samples/reversi/distill/gen"
	"github.com/shibukawa/ebigentserver/samples/reversi/reversi"
	"github.com/shibukawa/ebigentserver/session"
)

const corpusMatches = 60

// The pipeline reconstructs GreedyBot: every one of the corpus's
// decisions is reproduced by the mined decision list — the judgement
// vocabulary (data:derived-predicate) closes the distillation loop bit
// for bit, just as tic-tac-toe's coordinate vocabulary did.
func TestSynthesisReconstructsGreedy(t *testing.T) {
	lib, records, err := distill.Synthesize(corpusMatches)
	if err != nil {
		t.Fatal(err)
	}
	approved := lib.Approved()
	if len(approved) == 0 {
		t.Fatal("no approved chips")
	}
	if len(approved) != len(lib.Chips) {
		t.Fatalf("approved chips = %d of %d: some mined rules were dirty", len(approved), len(lib.Chips))
	}
	for _, c := range approved {
		if c.Counterexamples != 0 {
			t.Fatalf("chip %s carries counterexamples: %+v", c.Key(), c)
		}
		if len(c.Evidence) == 0 {
			t.Fatalf("chip %s has no evidence", c.Key())
		}
	}
	// Replay the decision list over the whole corpus.
	vocab := distill.Vocabulary()
	featIdx := map[string]int{}
	for i, f := range vocab.Features {
		featIdx[f.Name] = i
	}
	for _, r := range records {
		decided := ""
		for _, chip := range approved {
			if r.Bits[featIdx[chip.Condition]] {
				decided = chip.Action
				break
			}
		}
		if decided != r.Action {
			t.Fatalf("%s tick %d: list decides %s, greedy played %s", r.Episode, r.Tick, decided, r.Action)
		}
	}
}

// rule:regeneration-preserves-approved-nodes: re-running analysis diffs
// against the library instead of replacing it.
func TestRegenerationPreservesApprovedChips(t *testing.T) {
	lib, _, err := distill.Synthesize(corpusMatches)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := dir + "/chips.json"
	if err := lib.Save(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Same corpus, fresh proposals, merged into the loaded library:
	// nothing but "unchanged", and the file bytes do not move.
	loaded, err := behavior.LoadLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	records, err := distill.Corpus(corpusMatches)
	if err != nil {
		t.Fatal(err)
	}
	cands, _, err := behavior.SequentialCovering{}.Propose(distill.Vocabulary(), records)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range behavior.Merge(loaded, cands) {
		if d.Class != behavior.DiffUnchanged {
			t.Fatalf("diff class %s for %s→%s, want unchanged", d.Class, d.Candidate.Condition, d.Candidate.Action)
		}
	}
	if err := loaded.Save(path); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("regeneration moved the library file")
	}

	// A rejection is remembered and replayed, never resurrected.
	loaded.Chips[0].Rejected = true
	loaded.Chips[0].RejectReason = "test says no"
	rejKey := loaded.Chips[0].Key()
	sawRejected := false
	for _, d := range behavior.Merge(loaded, cands) {
		if d.Class == behavior.DiffRejectedAgain && d.Existing.Key() == rejKey {
			sawRejected = true
			if d.Existing.RejectReason != "test says no" {
				t.Fatalf("old reason lost: %+v", d.Existing)
			}
		}
	}
	if !sawRejected {
		t.Fatal("rejected chip not reported as matches_rejected")
	}

	// A proposal contradicting an approved chip surfaces as a conflict
	// and never touches the approved entry.
	conflictLib := &behavior.Library{Game: "reversi", Chips: []behavior.Chip{{
		Condition: "best_move_is_19", Action: "play_26", Approved: true, Priority: 0,
	}}}
	diff := behavior.Merge(conflictLib, []behavior.Candidate{{Condition: "best_move_is_19", Action: "play_19"}})
	if len(diff) != 1 || diff[0].Class != behavior.DiffConflict {
		t.Fatalf("diff = %+v, want one conflict", diff)
	}
	if len(conflictLib.Chips) != 1 || conflictLib.Chips[0].Action != "play_26" {
		t.Fatalf("approved chip mutated: %+v", conflictLib.Chips)
	}
}

// The committed generated sources are exactly what regeneration produces
// — the generated agent cannot drift from the library.
func TestGeneratedSourcesAreCurrent(t *testing.T) {
	lib, records, err := distill.Synthesize(corpusMatches)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := behavior.GenerateAgent(distill.Spec(), distill.Vocabulary(), lib)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("gen/agent_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(agent) != string(committed) {
		t.Fatal("gen/agent_gen.go is stale: regenerate it")
	}
	tests, err := behavior.GenerateTests(distill.TestSpec(), records, 24)
	if err != nil {
		t.Fatal(err)
	}
	committedTests, err := os.ReadFile("gen/agent_gen_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(tests) != string(committedTests) {
		t.Fatal("gen/agent_gen_test.go is stale: regenerate it")
	}
	// And the library artifact itself.
	committedLib, err := behavior.LoadLibrary("chips.json")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(lib)
	b, _ := json.Marshal(committedLib)
	if string(a) != string(b) {
		t.Fatal("chips.json is stale: regenerate it")
	}
}

// flow:automated-playtest: the agent generated from approved chips plays
// real sessions — and because the corpus was GreedyBot's own play, the
// distilled agent's matches against the same seeded opponents are
// move-for-move identical to the bot's.
func TestDistilledGreedyPassesAutomatedPlaytest(t *testing.T) {
	play := func(makeBlack func() session.Agent[reversi.Observation, reversi.Move]) func(int, uint64) (matchloop.Result, error) {
		return func(match int, seed uint64) (matchloop.Result, error) {
			s, err := session.New(session.Config[reversi.State, reversi.Move, reversi.Observation]{
				ID: "playtest", Slots: reversi.Slots(), RuleSet: reversi.RuleSet{}, Validator: reversi.Validator{},
				Seed: seed, Clock: func() int64 { return 0 },
			})
			if err != nil {
				return matchloop.Result{}, err
			}
			if err := s.OpenAdmission(); err != nil {
				return matchloop.Result{}, err
			}
			black := makeBlack()
			white := distill.NewRandomOpponent(seed)
			if err := s.Admit(reversi.SlotBlack, black); err != nil {
				return matchloop.Result{}, err
			}
			if err := s.Admit(reversi.SlotWhite, white); err != nil {
				return matchloop.Result{}, err
			}
			if err := s.Run(context.Background()); err != nil {
				return matchloop.Result{}, err
			}
			return matchloop.Result{
				Outcomes: map[session.SlotID]session.Terminal{
					reversi.SlotBlack: black.(interface{ Result() session.Terminal }).Result(),
				},
				Ticks: s.Tick(),
			}, nil
		}
	}

	const n = 30
	botSummary, err := matchloop.Run(n, 1000,
		play(func() session.Agent[reversi.Observation, reversi.Move] {
			return &outcomeAgent{inner: &reversi.GreedyBot{}}
		}))
	if err != nil {
		t.Fatal(err)
	}
	genSummary, err := matchloop.Run(n, 1000,
		play(func() session.Agent[reversi.Observation, reversi.Move] {
			return &outcomeAgent{inner: &gen.DistilledGreedy{}}
		}))
	if err != nil {
		t.Fatal(err)
	}
	if botSummary.Matches != n || genSummary.Matches != n {
		t.Fatalf("matches: %d vs %d", botSummary.Matches, genSummary.Matches)
	}
	// Identical policy, identical opponents, identical seeds: the
	// distilled agent's results equal the original's exactly.
	if botSummary.TotalTicks != genSummary.TotalTicks {
		t.Fatalf("durations diverged: %d vs %d ticks", botSummary.TotalTicks, genSummary.TotalTicks)
	}
	for _, term := range []session.Terminal{session.Win, session.Lose, session.Draw} {
		if botSummary.BySlot[reversi.SlotBlack][term] != genSummary.BySlot[reversi.SlotBlack][term] {
			t.Fatalf("%v counts diverged: bot %d vs distilled %d", term,
				botSummary.BySlot[reversi.SlotBlack][term], genSummary.BySlot[reversi.SlotBlack][term])
		}
	}
	t.Logf("playtest: %d matches, distilled win rate %.2f == greedy win rate %.2f, avg %d ticks",
		n, genSummary.WinRate(reversi.SlotBlack), botSummary.WinRate(reversi.SlotBlack),
		botSummary.TotalTicks/uint64(n))
}

// outcomeAgent wraps an agent and remembers its final terminal.
type outcomeAgent struct {
	inner session.Agent[reversi.Observation, reversi.Move]
	term  session.Terminal
}

func (a *outcomeAgent) Joined(s session.SlotID)       { a.inner.Joined(s) }
func (a *outcomeAgent) Observe(o reversi.Observation) { a.inner.Observe(o) }
func (a *outcomeAgent) Decide(ctx context.Context) (reversi.Move, bool) {
	return a.inner.Decide(ctx)
}
func (a *outcomeAgent) Ended(r session.Result) {
	a.term = r.Signal.Terminal
	a.inner.Ended(r)
}
func (a *outcomeAgent) Result() session.Terminal { return a.term }
