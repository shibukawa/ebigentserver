package distill_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/matchloop"
	"github.com/shibukawa/ebigentserver/samples/tictactoe/distill"
	"github.com/shibukawa/ebigentserver/samples/tictactoe/distill/gen"
	"github.com/shibukawa/ebigentserver/samples/tictactoe/ttt"
	"github.com/shibukawa/ebigentserver/session"
)

const corpusMatches = 200

// The pipeline reconstructs the sample bot: every one of the corpus's
// decisions is reproduced by the mined decision list — the distillation
// loop closes bit for bit.
func TestSynthesisReconstructsTheBot(t *testing.T) {
	lib, records, err := distill.Synthesize(corpusMatches)
	if err != nil {
		t.Fatal(err)
	}
	approved := lib.Approved()
	if len(approved) != 9 {
		t.Fatalf("approved chips = %d, want 9", len(approved))
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
			t.Fatalf("%s tick %d: list decides %s, bot played %s", r.Episode, r.Tick, decided, r.Action)
		}
	}
}

// Phase 7 acceptance 2 (rule:regeneration-preserves-approved-nodes):
// re-running analysis diffs against the library instead of replacing it.
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
	conflictLib := &behavior.Library{Game: "tictactoe", Chips: []behavior.Chip{{
		Condition: "cell_0_empty", Action: "play_4", Approved: true, Priority: 0,
	}}}
	diff := behavior.Merge(conflictLib, []behavior.Candidate{{Condition: "cell_0_empty", Action: "play_0"}})
	if len(diff) != 1 || diff[0].Class != behavior.DiffConflict {
		t.Fatalf("diff = %+v, want one conflict", diff)
	}
	if len(conflictLib.Chips) != 1 || conflictLib.Chips[0].Action != "play_4" {
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
	tests, err := behavior.GenerateTests(distill.Spec(), records, 24)
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

// Phase 7 acceptance 1 (flow:automated-playtest): the agent generated
// from approved chips plays real sessions — and because the corpus was
// the bot's own play, the distilled agent's matches are move-for-move
// identical to the bot's.
func TestDistilledAgentPassesAutomatedPlaytest(t *testing.T) {
	play := func(makeX func() session.Agent[ttt.Sight, ttt.Move]) func(int, uint64) (matchloop.Result, error) {
		return func(match int, seed uint64) (matchloop.Result, error) {
			s, err := session.New(session.Config[ttt.State, ttt.Move, ttt.Sight]{
				ID: "playtest", Slots: ttt.Slots(), RuleSet: ttt.RuleSet{}, Validator: ttt.Validator{},
				Seed: seed, Clock: func() int64 { return 0 },
			})
			if err != nil {
				return matchloop.Result{}, err
			}
			if err := s.OpenAdmission(); err != nil {
				return matchloop.Result{}, err
			}
			x := makeX()
			o := distill.NewRandomOpponent(seed)
			if err := s.Admit(ttt.SlotX, x); err != nil {
				return matchloop.Result{}, err
			}
			if err := s.Admit(ttt.SlotO, o); err != nil {
				return matchloop.Result{}, err
			}
			if err := s.Run(context.Background()); err != nil {
				return matchloop.Result{}, err
			}
			return matchloop.Result{
				Outcomes: map[session.SlotID]session.Terminal{
					ttt.SlotX: x.(interface{ Result() session.Terminal }).Result(),
				},
				Ticks: s.Tick(),
			}, nil
		}
	}

	const n = 50
	botSummary, err := matchloop.Run(n, 1000,
		play(func() session.Agent[ttt.Sight, ttt.Move] { return &outcomeAgent{inner: &ttt.Bot{}} }))
	if err != nil {
		t.Fatal(err)
	}
	genSummary, err := matchloop.Run(n, 1000,
		play(func() session.Agent[ttt.Sight, ttt.Move] { return &outcomeAgent{inner: &gen.DistilledAgent{}} }))
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
		if botSummary.BySlot[ttt.SlotX][term] != genSummary.BySlot[ttt.SlotX][term] {
			t.Fatalf("%v counts diverged: bot %d vs distilled %d", term,
				botSummary.BySlot[ttt.SlotX][term], genSummary.BySlot[ttt.SlotX][term])
		}
	}
	t.Logf("playtest: %d matches, distilled win rate %.2f == bot win rate %.2f, avg %d ticks",
		n, genSummary.WinRate(ttt.SlotX), botSummary.WinRate(ttt.SlotX),
		botSummary.TotalTicks/uint64(n))
}

// outcomeAgent wraps an agent and remembers its final terminal.
type outcomeAgent struct {
	inner session.Agent[ttt.Sight, ttt.Move]
	term  session.Terminal
}

func (a *outcomeAgent) Joined(s session.SlotID)                     { a.inner.Joined(s) }
func (a *outcomeAgent) Observe(o ttt.Sight)                         { a.inner.Observe(o) }
func (a *outcomeAgent) Decide(ctx context.Context) (ttt.Move, bool) { return a.inner.Decide(ctx) }
func (a *outcomeAgent) Ended(r session.Result) {
	a.term = r.Signal.Terminal
	a.inner.Ended(r)
}
func (a *outcomeAgent) Result() session.Terminal { return a.term }

// One library, two personalities: the loadout's tactic selector claims
// the center while the plain decision list stays leftmost — assembled
// without any new analysis (decision:shared-chip-library,
// concept:tactic-selector).
func TestLoadoutAssemblesADifferentPersonality(t *testing.T) {
	var empty ttt.Sight // blank board
	if m, ok := gen.Decide(empty); !ok || m.Cell != 0 {
		t.Fatalf("base list opens with %v, want cell 0", m)
	}
	if m, ok := gen.TacticDecide(empty); !ok || m.Cell != 4 {
		t.Fatalf("center-first loadout opens with %v, want cell 4", m)
	}
	// Once the center is gone, the tactic falls through to leftmost.
	taken := empty
	taken.Board[4] = ttt.MarkO
	if m, ok := gen.TacticDecide(taken); !ok || m.Cell != 0 {
		t.Fatalf("fallback tactic plays %v, want cell 0", m)
	}
	// The loadout agent completes real matches.
	sum, err := matchloop.Run(10, 77, func(match int, seed uint64) (matchloop.Result, error) {
		s, err := session.New(session.Config[ttt.State, ttt.Move, ttt.Sight]{
			ID: "loadout", Slots: ttt.Slots(), RuleSet: ttt.RuleSet{}, Validator: ttt.Validator{},
			Seed: seed, Clock: func() int64 { return 0 },
		})
		if err != nil {
			return matchloop.Result{}, err
		}
		if err := s.OpenAdmission(); err != nil {
			return matchloop.Result{}, err
		}
		x := &outcomeAgent{inner: &gen.TacticAgent{}}
		if err := s.Admit(ttt.SlotX, x); err != nil {
			return matchloop.Result{}, err
		}
		if err := s.Admit(ttt.SlotO, distill.NewRandomOpponent(seed)); err != nil {
			return matchloop.Result{}, err
		}
		if err := s.Run(context.Background()); err != nil {
			return matchloop.Result{}, err
		}
		return matchloop.Result{
			Outcomes: map[session.SlotID]session.Terminal{ttt.SlotX: x.Result()},
			Ticks:    s.Tick(),
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Matches != 10 {
		t.Fatalf("matches = %d", sum.Matches)
	}
}

// The committed loadout source is exactly what regeneration produces.
func TestGeneratedLoadoutIsCurrent(t *testing.T) {
	lib, _, err := distill.Synthesize(corpusMatches)
	if err != nil {
		t.Fatal(err)
	}
	src, err := behavior.GenerateLoadoutAgent(distill.Spec(), distill.Vocabulary(), lib,
		distill.CenterFirstLoadout(), "TacticDecide", "TacticAgent")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("gen/loadout_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != string(committed) {
		t.Fatal("gen/loadout_gen.go is stale: regenerate it")
	}
}
