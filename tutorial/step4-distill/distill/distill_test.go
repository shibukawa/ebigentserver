package distill_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill/gen"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill/pred"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/game"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/msg"
)

// TestTheNaiveVocabularyQuietlyLosesMostOfTheBot is the finding step 4
// is built around.
//
// Nothing errors. Propose reports no uncovered decisions. Every number
// on the screen looks like success — and the chips that survive approval
// account for barely half of what the bot did, because the rules that
// would have covered the rest carry counterexamples and counterexamples
// are never approved. An agent generated from this library is silent in
// exactly the positions the bot handled by knowing something.
func TestTheNaiveVocabularyQuietlyLosesMostOfTheBot(t *testing.T) {
	lib, records, err := distill.Synthesize(distill.CorpusMatches/4, distill.Naive())
	if err != nil {
		t.Fatal(err)
	}
	approved := lib.Approved()
	if len(approved) == len(lib.Chips) {
		t.Fatalf("every one of %d chips came back clean; the naive vocabulary was supposed to fail", len(lib.Chips))
	}
	covered := distill.Covered(lib, records)
	if covered >= len(records) {
		t.Fatal("the naive vocabulary covered the whole corpus, which would make step 4 pointless")
	}
	// Held to a range rather than a number: the point is that a large
	// fraction goes missing, not that it is exactly this fraction.
	share := float64(covered) / float64(len(records))
	if share < 0.4 || share > 0.8 {
		t.Fatalf("approved chips covered %.1f%% of decisions, outside the expected 40-80%%", 100*share)
	}
	t.Logf("naive: %d chips, %d approved, %d/%d decisions covered (%.1f%%)",
		len(lib.Chips), len(approved), covered, len(records), 100*share)
}

// TestJudgementVocabularyExplainsEveryDecision is the same corpus under
// names for the distinctions the board does not spell out.
//
// The interesting part is not that it covers everything. It is that
// nobody told the miner that winning outranks blocking, or that blocking
// outranks taking the centre. Sequential covering prefers rules the
// corpus never contradicts, and the only ordering in which these rules
// are contradiction-free is the one the bot was written with — so the
// priority comes back out of the recording on its own.
func TestJudgementVocabularyExplainsEveryDecision(t *testing.T) {
	lib, records, err := distill.Synthesize(distill.CorpusMatches, distill.Judgement())
	if err != nil {
		t.Fatal(err)
	}
	for _, chip := range lib.Chips {
		if chip.Counterexamples != 0 {
			t.Errorf("chip %s has %d counterexamples", chip.Key(), chip.Counterexamples)
		}
	}
	if got := distill.Covered(lib, records); got != len(records) {
		t.Fatalf("approved chips covered %d of %d decisions", got, len(records))
	}
	t.Logf("judgement: %d chips, all approved, %d/%d decisions covered", len(lib.Chips), len(records), len(records))
}

// TestGeneratedAgentPlaysExactlyLikeTheBot is the closing claim, and it
// is checked by exhaustion rather than by sampling.
//
// Every position the bot can reach as X is walked: the bot answers, the
// opponent replies every legal way, and at each of the bot's turns the
// generated Decide is asked the same question. A corpus of two hundred
// games visits a fraction of these, so agreement here is a stronger
// statement than the coverage number the mining reported.
func TestGeneratedAgentPlaysExactlyLikeTheBot(t *testing.T) {
	positions := 0
	walkReachable(t, func(world msg.TTTWorld) {
		positions++
		sight := game.RuleSet{}.Project(&world, game.SlotX)

		bot := &game.Bot{}
		bot.Observe(sight)
		want, wantOK := bot.Decide(context.Background())

		got, gotOK := gen.Decide(sight)
		if gotOK != wantOK {
			t.Fatalf("board %v: generated agent answered %v, bot answered %v", world.Cells, gotOK, wantOK)
		}
		if wantOK && got != want {
			t.Fatalf("board %v: generated agent played %d, bot played %d", world.Cells, got.Cell, want.Cell)
		}
	})
	if positions == 0 {
		t.Fatal("the walk visited nothing")
	}
	t.Logf("agreed on all %d positions the bot can reach", positions)
}

// TestASmallCorpusIsMissingRules is the first of three sizes, and the
// three together are why step 5 exists.
//
// A quarter of the canonical corpus mines a list with no
// counterexamples and reports every recorded decision explained. Walked
// over the positions the bot can actually reach, that list is silent on
// several: there are boards the bot answers and the library has no rule
// for. The corpus had no situation for the missing rules, so it also had
// no evidence that they were missing.
func TestASmallCorpusIsMissingRules(t *testing.T) {
	lib, records := mine(t, distill.CorpusMatches/4)
	silent, wrong := disagreements(t, lib)
	if len(silent) == 0 {
		t.Fatalf("%d games left no hole, so the step has nothing to show", distill.CorpusMatches/4)
	}
	t.Logf("%d games: %d chips, %d/%d decisions covered, silent on %d boards (e.g. %v), wrong on %d",
		distill.CorpusMatches/4, len(lib.Approved()), distill.Covered(lib, records), len(records),
		len(silent), silent[0], len(wrong))
}

// TestAHalfCorpusHasEveryRuleAndStillPlaysWrong is the one worth
// stopping at.
//
// At half the corpus the rule count reaches its final value: doubling
// the games again finds no new rules. Every signal available from inside
// the pipeline says the work is done — no counterexamples, full
// coverage, a list that has stopped growing. And the agent still answers
// at least one reachable board differently from the bot, because a rule
// sits above a rule that should outrank it.
//
// The order of a decision list is not information the corpus states. It
// falls out of which rules the corpus manages to contradict, so a
// position that never came up is an ordering nobody checked.
func TestAHalfCorpusHasEveryRuleAndStillPlaysWrong(t *testing.T) {
	half, _ := mine(t, distill.CorpusMatches/2)
	full, _ := mine(t, distill.CorpusMatches)
	if len(half.Approved()) != len(full.Approved()) {
		t.Fatalf("half the corpus mined %d chips and the whole one %d; they were supposed to agree on the count",
			len(half.Approved()), len(full.Approved()))
	}
	silent, wrong := disagreements(t, half)
	if len(silent) != 0 {
		t.Fatalf("%d games still leave %d silent boards; that is the previous test's finding, not this one",
			distill.CorpusMatches/2, len(silent))
	}
	if len(wrong) == 0 {
		t.Fatalf("%d games played every reachable board correctly, so the ordering point has nothing to stand on",
			distill.CorpusMatches/2)
	}
	t.Logf("%d games: same %d chips as the full corpus, and %d boards answered wrongly (e.g. %v)",
		distill.CorpusMatches/2, len(half.Approved()), len(wrong), wrong[0])
}

// TestTheCanonicalCorpusAgreesEverywhere is the third size, and the one
// the committed agent came from. What the last doubling bought was not
// knowledge but ordering.
func TestTheCanonicalCorpusAgreesEverywhere(t *testing.T) {
	lib, _ := mine(t, distill.CorpusMatches)
	silent, wrong := disagreements(t, lib)
	if len(silent) > 0 || len(wrong) > 0 {
		t.Fatalf("%d games leave %d silent and %d wrong boards", distill.CorpusMatches, len(silent), len(wrong))
	}
}

// mine synthesizes and checks the things every size has in common: no
// counterexamples, and a corpus that calls itself fully explained.
func mine(t *testing.T, matches int) (*behavior.Library, []behavior.Record) {
	t.Helper()
	lib, records, err := distill.Synthesize(matches, distill.Judgement())
	if err != nil {
		t.Fatal(err)
	}
	for _, chip := range lib.Chips {
		if chip.Counterexamples != 0 {
			t.Fatalf("%d games: chip %s has counterexamples; every size here is supposed to look clean",
				matches, chip.Key())
		}
	}
	if got := distill.Covered(lib, records); got != len(records) {
		t.Fatalf("%d games: covered %d of %d; every size here is supposed to report itself complete",
			matches, got, len(records))
	}
	return lib, records
}

// disagreements walks the bot's reachable domain and splits the boards a
// library gets wrong into the two ways it can: no rule fired, or the
// wrong one did.
func disagreements(t *testing.T, lib *behavior.Library) (silent, wrong [][]uint8) {
	t.Helper()
	walkReachable(t, func(world msg.TTTWorld) {
		obs := game.RuleSet{}.Project(&world, game.SlotX)
		bot := &game.Bot{}
		bot.Observe(obs)
		want, ok := bot.Decide(context.Background())
		if !ok {
			return
		}
		got, gotOK := interpret(lib, obs)
		switch {
		case !gotOK:
			silent = append(silent, append([]uint8(nil), world.Cells...))
		case got != want:
			wrong = append(wrong, append([]uint8(nil), world.Cells...))
		}
	})
	return silent, wrong
}

// silentPositions walks the bot's reachable domain and returns every
// board where the library has no rule and the bot had a move.
func silentPositions(t *testing.T, lib *behavior.Library) [][]uint8 {
	t.Helper()
	var out [][]uint8
	walkReachable(t, func(world msg.TTTWorld) {
		obs := game.RuleSet{}.Project(&world, game.SlotX)
		bot := &game.Bot{}
		bot.Observe(obs)
		if _, ok := bot.Decide(context.Background()); !ok {
			return
		}
		if _, ok := interpret(lib, obs); !ok {
			out = append(out, append([]uint8(nil), world.Cells...))
		}
	})
	return out
}

// interpret runs a library the way the generated switch would, without
// generating it, so a vocabulary can be judged before it is compiled.
func interpret(lib *behavior.Library, obs game.Sight) (msg.Move, bool) {
	for _, chip := range lib.Approved() {
		name, cell, ok := split(chip.Condition)
		if !ok {
			continue
		}
		var holds bool
		switch name {
		case "winning_move_is":
			holds = pred.WinningMoveIs(obs, cell)
		case "blocking_move_is":
			holds = pred.BlockingMoveIs(obs, cell)
		case "preferred_cell_is":
			holds = pred.PreferredCellIs(obs, cell)
		}
		if !holds {
			continue
		}
		if _, play, ok := split(chip.Action); ok {
			return msg.Move{Cell: uint8(play)}, true
		}
	}
	return msg.Move{}, false
}

// split parses a vocabulary name ending in an underscore and a digit.
func split(name string) (string, int, bool) {
	i := strings.LastIndex(name, "_")
	if i < 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(name[i+1:])
	if err != nil {
		return "", 0, false
	}
	return name[:i], n, true
}

// TestMinerAndRuntimeAgree keeps the two halves of every predicate in
// step.
//
// A feature is written twice: once as Eval, which reads recorded JSON
// while mining, and once as GoExpr, which the generated agent compiles
// into a call on the live sight. Nothing makes them agree. If they drift,
// the library still validates, the generated code still builds, and the
// agent plays a policy the corpus never contained — the one failure in
// this pipeline that leaves no trace anywhere else.
func TestMinerAndRuntimeAgree(t *testing.T) {
	vocab := distill.Judgement()
	checked := 0
	walkReachable(t, func(world msg.TTTWorld) {
		sight := game.RuleSet{}.Project(&world, game.SlotX)
		raw, err := json.Marshal(sight)
		if err != nil {
			t.Fatal(err)
		}
		for i, f := range vocab.Features {
			mined, err := f.Eval(raw)
			if err != nil {
				t.Fatalf("%s: %v", f.Name, err)
			}
			if mined != runtimePredicate(t, i, sight) {
				t.Fatalf("board %v: %s mines as %v but evaluates as %v at runtime",
					world.Cells, f.Name, mined, !mined)
			}
			checked++
		}
	})
	t.Logf("%d predicate evaluations agreed between miner and runtime", checked)
}

// runtimePredicate is the GoExpr side of feature i, called directly.
// The vocabulary is three blocks of nine in a known order, which is what
// lets an index stand in for the generated call.
func runtimePredicate(t *testing.T, i int, obs game.Sight) bool {
	t.Helper()
	switch cell := i % 9; {
	case i < 9:
		return pred.WinningMoveIs(obs, cell)
	case i < 18:
		return pred.BlockingMoveIs(obs, cell)
	case i < 27:
		return pred.PreferredCellIs(obs, cell)
	}
	t.Fatalf("feature index %d is outside the judgement vocabulary", i)
	return false
}

// walkReachable visits every position the bot faces as X, from the empty
// board, with the opponent replying every legal way.
func walkReachable(t *testing.T, visit func(msg.TTTWorld)) {
	t.Helper()
	var rules game.RuleSet
	var step func(world msg.TTTWorld)
	step = func(world msg.TTTWorld) {
		if world.Over {
			return
		}
		acting := rules.ActingSlots(&world)
		if len(acting) != 1 {
			t.Fatalf("%d seats acting", len(acting))
		}
		seat := acting[0]

		if seat == game.SlotX {
			visit(world)
			bot := &game.Bot{}
			bot.Observe(rules.Project(&world, seat))
			move, ok := bot.Decide(context.Background())
			if !ok {
				t.Fatalf("the bot passed on its own turn: %v", world.Cells)
			}
			next := clone(world)
			rules.Apply(&next, seat, move)
			step(next)
			return
		}

		// The opponent is every legal reply, which is what makes this a
		// proof over the bot's whole reachable domain rather than over
		// one opponent's habits.
		for _, cell := range rules.Project(&world, seat).Legal {
			next := clone(world)
			rules.Apply(&next, seat, msg.Move{Cell: uint8(cell)})
			step(next)
		}
	}
	step(rules.Start(0))
}

// clone deep-copies a board so a branch cannot write into its sibling.
func clone(w msg.TTTWorld) msg.TTTWorld {
	c := w
	c.Cells = append([]uint8(nil), w.Cells...)
	c.Line = append([]uint8(nil), w.Line...)
	return c
}

// update rewrites the committed generated sources instead of comparing
// against them:
//
//	go test ./tutorial/step4-distill/distill -update
//
// The regeneration lives here rather than in a command of its own so
// that what is written and what is compared are the same call.
var update = flag.Bool("update", false, "rewrite the committed generated sources instead of comparing against them")

// TestGeneratedSourcesAreCurrent keeps the committed agent honest: it is
// what this corpus produces today, not what it produced once.
func TestGeneratedSourcesAreCurrent(t *testing.T) {
	c, err := distill.Compile()
	if err != nil {
		t.Fatal(err)
	}
	files, err := c.Sources()
	if err != nil {
		t.Fatal(err)
	}
	// The command writes these and this compares them, and both ask the
	// same call for the bytes. That is the whole of why the loop closes:
	// when the two built their own corpora, a regeneration could leave
	// this red and neither running it again nor rerunning the command
	// would fix it.
	if *update {
		if err := c.Write("gen"); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d files under gen/", len(files))
		return
	}
	for name, generated := range files {
		committed, err := os.ReadFile(filepath.Join("gen", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(generated) != string(committed) {
			t.Fatalf("gen/%s is stale; regenerate it with: ebigent distill", name)
		}
	}
}

// TestTheDistilledAgentPlaysTheSameMatches is the validate step of
// flow:behavior-tree-synthesis, and it is a different claim from the
// position walk above.
//
// The walk asks whether the two policies agree. This asks whether the
// generated one can be seated at all: it goes through session.New,
// admission, and a real step loop, so an agent that answers correctly
// but cannot hold a seat fails here and nowhere else. Same seeds, same
// opponent, and every match has to end the same way after the same
// number of ticks.
func TestTheDistilledAgentPlaysTheSameMatches(t *testing.T) {
	const matches = 60
	for seed := uint64(0); seed < matches; seed++ {
		want, err := distill.Playtest(seed, &game.Bot{})
		if err != nil {
			t.Fatalf("seed %d, hand-written: %v", seed, err)
		}
		got, err := distill.Playtest(seed, &gen.Distilled{})
		if err != nil {
			t.Fatalf("seed %d, distilled: %v", seed, err)
		}
		if got != want {
			t.Fatalf("seed %d: distilled ended %v after %d ticks, hand-written ended %v after %d",
				seed, got.Terminal, got.Ticks, want.Terminal, want.Ticks)
		}
	}
	t.Logf("%d matches, identical outcome and length in every one", matches)
}

// TestCurateThenMineMeasuresTheHoldout closes the curation loop over a
// real recording: curate splits and caps the corpus, mining sees only
// the train side, and the holdout answers with play the miner never saw.
//
// The counts have to reconcile — every holdout decision lands in exactly
// one of covered, misplayed, or silent — because a bucket that quietly
// dropped rows would make the honest number as unfounded as the
// flattering one it exists to correct (requirement:corpus-curation).
func TestCurateThenMineMeasuresTheHoldout(t *testing.T) {
	root := t.TempDir()
	if err := distill.Record(root, 60, distill.CorpusSeed); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "curated")
	rep, err := behavior.Curate(root, out, behavior.CurateOptions{
		Filter:  behavior.RowFilter{AgentKind: distill.KindTactic},
		Cap:     3,
		Holdout: 0.2,
		Seed:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.TrainDropped == 0 {
		t.Fatal("sixty deterministic openings and the cap dropped nothing; the aggregation is not aggregating")
	}
	if rep.HoldoutEpisodes == 0 || rep.TrainEpisodes == 0 {
		t.Fatalf("split %d/%d put everything on one side", rep.TrainEpisodes, rep.HoldoutEpisodes)
	}
	if len(rep.Conflicts) != 0 {
		t.Fatalf("the bot is deterministic, so %d conflicts can only be a keying bug", len(rep.Conflicts))
	}

	c, err := distill.CompileFrom(filepath.Join(out, "train"))
	if err != nil {
		t.Fatal(err)
	}
	hold, ok, err := distill.EvaluateHoldout(filepath.Join(out, "train"), c, distill.Corpus)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("EvaluateHoldout did not find the holdout beside the train side")
	}
	total := len(hold.Covered) + len(hold.Misplayed) + len(hold.Silent)
	if total != rep.HoldoutRows {
		t.Fatalf("evaluated %d holdout decisions, curate reported %d", total, rep.HoldoutRows)
	}
	if _, err := os.Stat(filepath.Join(out, "gaps.jsonl")); err != nil {
		t.Fatalf("no gaps.jsonl beside the curated corpus: %v", err)
	}
	t.Logf("curate: %d tactic rows → %d train after cap 3, %d holdout; holdout: %d covered, %d misplayed, %d silent",
		rep.TrainRows+rep.HoldoutRows, rep.TrainKept, rep.HoldoutRows,
		len(hold.Covered), len(hold.Misplayed), len(hold.Silent))
}

// TestTheCoinIsPolicyMixingMadeVisible pins the reason curate lists
// conflicts instead of resolving them. The coin answers one situation
// several ways — which is exactly what a human corpus does — and the
// deterministic miner would book every minority answer as a
// counterexample. Curate's job is to put that mixture on the table
// before mining turns it into rejections.
func TestTheCoinIsPolicyMixingMadeVisible(t *testing.T) {
	root := t.TempDir()
	if err := distill.Record(root, 60, distill.CorpusSeed); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "curated")
	rep, err := behavior.Curate(root, out, behavior.CurateOptions{
		Filter: behavior.RowFilter{AgentKind: distill.KindCoin},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Conflicts) == 0 {
		t.Fatal("a random opponent answered every situation one way across sixty games?")
	}
	t.Logf("coin: %d situations, %d answered more than one way", rep.Situations, len(rep.Conflicts))
}

// TestYourCopyMinesFromHumanRows drives the human path end to end on a
// fixture corpus whose rows carry agent_kind human: curate keeps only
// those, CompileYours mines every seat of what survived, and the sources
// render as package genhuman with the You agent — the same generator,
// under your name.
func TestYourCopyMinesFromHumanRows(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "you-0000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"stream":"decisions","schema_version":1,"episode_id":"you-0000"}
{"tick":1,"slot":1,"agent_kind":"human","sight":{"you":1,"mark":"X","turn":1,"cells":["-","-","-","-","-","-","-","-","-"],"legal":[0,1,2,3,4,5,6,7,8]},"action":{"cell":4}}
{"tick":3,"slot":2,"agent_kind":"coin","sight":{"you":2,"mark":"O","turn":2,"cells":["-","-","-","-","X","-","-","-","-"],"legal":[0,1,2,3,5,6,7,8]},"action":{"cell":1}}
{"tick":5,"slot":1,"agent_kind":"human","sight":{"you":1,"mark":"X","turn":1,"cells":["-","O","-","-","X","-","-","-","-"],"legal":[0,2,3,5,6,7,8]},"action":{"cell":0}}
`
	if err := os.WriteFile(filepath.Join(dir, "decisions.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "curated")
	rep, err := behavior.Curate(root, out, behavior.CurateOptions{
		Filter: behavior.RowFilter{AgentKind: "human"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.TrainKept != 2 || rep.FilteredOut != 1 {
		t.Fatalf("kept %d filtered %d; want the two human rows alone", rep.TrainKept, rep.FilteredOut)
	}

	c, err := distill.CompileYours(filepath.Join(out, "train"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := c.SourcesAs(distill.HumanSpec(), distill.HumanTestSpec())
	if err != nil {
		t.Fatal(err)
	}
	agent := string(files["agent_gen.go"])
	if !strings.Contains(agent, "package genhuman") || !strings.Contains(agent, "type You struct") {
		t.Fatalf("human sources did not render as genhuman.You:\n%s", agent)
	}
}

// TestUnderstudyAnswersWhereTheCopyIsSilent pins the seat the -opponent
// you flag fills: the primary answers when it can, the backup answers
// when it cannot, and the gap count says how often the corpus fell
// short.
func TestUnderstudyAnswersWhereTheCopyIsSilent(t *testing.T) {
	u := &distill.Understudy{
		Primary: silentAgent{},
		Backup:  distill.Opponents()[distill.KindCoin](1),
	}
	u.Joined(game.SlotX)
	u.Observe(game.Sight{Legal: []int{3, 5}})
	m, ok := u.Decide(context.Background())
	if !ok {
		t.Fatal("the understudy did not answer for a silent primary")
	}
	if m.Cell != 3 && m.Cell != 5 {
		t.Fatalf("understudy played %d, outside the legal cells", m.Cell)
	}
	if u.Gaps != 1 {
		t.Fatalf("gaps %d, want 1", u.Gaps)
	}
}

// silentAgent is the committed genhuman placeholder in miniature: it
// never answers.
type silentAgent struct{}

func (silentAgent) Joined(session.SlotID)                     {}
func (silentAgent) Observe(game.Sight)                        {}
func (silentAgent) Decide(context.Context) (msg.Move, bool)   { return msg.Move{}, false }
func (silentAgent) Ended(session.Result)                      {}
