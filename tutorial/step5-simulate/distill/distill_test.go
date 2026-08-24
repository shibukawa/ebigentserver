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
	"time"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/distill"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/distill/gen"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/distill/pred"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/game"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/msg"
)

type agent = session.Agent[game.Sight, msg.Move]

func perfect() agent { return &distill.Perfect{Principled: true} }

// TestRotatingTheOpponentBeatsPlayingMore is the first half of step 5,
// and it is the answer to what step 4 ended on.
//
// Step 4 needed eight hundred games against a coin, and the ordering it
// arrived at was correct because the frequencies fell that way rather
// than because anything contradicted the alternative. Rotating the
// opponent reaches a better-explained corpus at a quarter of that,
// because the positions that settle an ordering come from players that
// produce them.
func TestRotatingTheOpponentBeatsPlayingMore(t *testing.T) {
	rotated := share(t, distill.CorpusMatches/2, distill.Judgement(), perfect, distill.RoundRobin())
	coin := share(t, distill.CorpusMatches*2, distill.Judgement(), perfect, distill.RandomOnly())

	if rotated <= coin {
		t.Fatalf("rotating over %d games explained %.1f%% and a coin over %d explained %.1f%%; rotation was supposed to win",
			distill.CorpusMatches/2, 100*rotated, distill.CorpusMatches*2, 100*coin)
	}
	t.Logf("rotated %d games: %.1f%% explained; coin %d games: %.1f%%",
		distill.CorpusMatches/2, 100*rotated, distill.CorpusMatches*2, 100*coin)
}

// TestAPerfectTeacherCannotBeDistilled is the second half, and it is a
// negative result reported as one.
//
// Twelve runs: two vocabularies, two pairing policies, three corpus
// sizes. None of them explains more than four fifths of what a perfect
// player did, and the gap does not close in the direction of any of the
// three knobs. More games move it by a point. Better words move it by
// seven. Better opponents move it by two.
//
// The reason is not that the teacher is clever. Every opening move of
// tic-tac-toe draws under perfect play, so the search has nine equally
// good answers and something arbitrary picks one. A predicate can only
// state facts about the position, and the position does not determine
// that choice — so there is nothing there to distill, at any volume.
func TestAPerfectTeacherCannotBeDistilled(t *testing.T) {
	if testing.Short() {
		t.Skip("plays several thousand games")
	}
	best := 0.0
	for _, vocab := range []struct {
		name string
		make func() *behavior.Vocabulary
	}{{"judgement", distill.Judgement}, {"fork", distill.Fork}} {
		for _, pairing := range []distill.Pairing{distill.RandomOnly(), distill.RoundRobin()} {
			for _, n := range []int{distill.CorpusMatches / 2, distill.CorpusMatches, distill.CorpusMatches * 2} {
				got := share(t, n, vocab.make(), perfect, pairing)
				if got > best {
					best = got
				}
				t.Logf("%-9s %-11s %4d games  %.1f%% explained", vocab.name, pairing.Name, n, 100*got)
			}
		}
	}
	if best >= 0.9 {
		t.Fatalf("the best run explained %.1f%%; a perfect teacher was supposed to stay out of reach", 100*best)
	}
	t.Logf("best of twelve runs: %.1f%%", 100*best)
}

// TestForkWordsTradeSilenceForConfidence is the part of the previous
// result worth separating out, because it is the opposite of what adding
// vocabulary is supposed to do.
//
// The fork terms let the miner explain moves it previously had no word
// for, and the agent stops going quiet. It does not start being right —
// it starts being wrong out loud. A richer vocabulary produces more
// rules that the corpus never contradicts, and a rule the corpus never
// contradicts is not thereby correct.
func TestForkWordsTradeSilenceForConfidence(t *testing.T) {
	plain, _, err := distill.MineFrom(distill.CorpusMatches, distill.Judgement(), perfect, distill.RandomOnly())
	if err != nil {
		t.Fatal(err)
	}
	forked, _, err := distill.MineFrom(distill.CorpusMatches, distill.Fork(), perfect, distill.RandomOnly())
	if err != nil {
		t.Fatal(err)
	}
	plainSilent, plainWrong := disagreements(t, plain, perfect)
	forkSilent, forkWrong := disagreements(t, forked, perfect)

	if forkSilent >= plainSilent {
		t.Fatalf("fork words left %d silent boards against %d without them; they were supposed to reduce silence",
			forkSilent, plainSilent)
	}
	if forkWrong <= plainWrong {
		t.Fatalf("fork words left %d wrong boards against %d without them; the trade is the finding",
			forkWrong, plainWrong)
	}
	t.Logf("judgement: %d silent, %d wrong.  fork: %d silent, %d wrong",
		plainSilent, plainWrong, forkSilent, forkWrong)
}

// TestTheSearchCostsWhatTheListDoesNot is the other half of why a
// perfect teacher is not the answer, and the half that would matter even
// if it could be distilled.
//
// rule:generated-agent-code-is-deterministic bounds a predicate by the
// tick: it runs per agent per tick, so generation rejects unbounded
// scans. A search is the unbounded scan. Seating the teacher directly is
// always possible — it is an ordinary agent — and on a three by three
// board it already costs a visible fraction of a tick for one seat.
func TestTheSearchCostsWhatTheListDoesNot(t *testing.T) {
	var rules game.RuleSet
	world := rules.Start(0)
	obs := rules.Project(&world, game.SlotX)

	teacher := perfect()
	teacher.Observe(obs)
	student := &game.Bot{}
	student.Observe(obs)

	slow := timeDecide(t, func() { teacher.Decide(context.Background()) }, 20)
	fast := timeDecide(t, func() { student.Decide(context.Background()) }, 20000)

	ratio := float64(slow) / float64(fast)
	if ratio < 1000 {
		t.Fatalf("the search was only %.0fx the decision list; the cost point needs a real gap", ratio)
	}
	t.Logf("search %v per decision, list %v — %.0fx", slow, fast, ratio)
}

// timeDecide is a crude per-call duration. It is not a benchmark and does
// not need to be: the gap being measured is orders of magnitude.
func timeDecide(t *testing.T, call func(), n int) time.Duration {
	t.Helper()
	start := time.Now()
	for i := 0; i < n; i++ {
		call()
	}
	return time.Since(start) / time.Duration(n)
}

// TestGeneratedAgentPlaysExactlyLikeTheBot carries step 4's claim
// forward against the corpus this step records: every position the
// teacher can reach, walked, with the committed agent asked the same
// question.
func TestGeneratedAgentPlaysExactlyLikeTheBot(t *testing.T) {
	positions := 0
	walk(t, distill.Tactic, func(obs game.Sight) {
		positions++
		bot := &game.Bot{}
		bot.Observe(obs)
		want, wantOK := bot.Decide(context.Background())
		got, gotOK := gen.Decide(obs)
		if gotOK != wantOK || (wantOK && got != want) {
			t.Fatalf("board %v: generated %v/%v, bot %v/%v", obs.Cells, got, gotOK, want, wantOK)
		}
	})
	t.Logf("agreed on all %d positions the teacher can reach", positions)
}

// share reports how much of a corpus the approved chips explain.
func share(t *testing.T, matches int, v *behavior.Vocabulary, teacher func() agent, p distill.Pairing) float64 {
	t.Helper()
	lib, records, err := distill.MineFrom(matches, v, teacher, p)
	if err != nil {
		t.Fatal(err)
	}
	covered := 0
	for _, chip := range lib.Approved() {
		covered += chip.Coverage
	}
	return float64(covered) / float64(len(records))
}

// disagreements walks the teacher's reachable domain and counts the
// boards a library has no rule for and the boards it answers wrongly.
func disagreements(t *testing.T, lib *behavior.Library, teacher func() agent) (silent, wrong int) {
	t.Helper()
	walk(t, teacher, func(obs game.Sight) {
		a := teacher()
		a.Observe(obs)
		want, ok := a.Decide(context.Background())
		if !ok {
			return
		}
		got, gotOK := interpret(lib, obs)
		switch {
		case !gotOK:
			silent++
		case got != want:
			wrong++
		}
	})
	return silent, wrong
}

// walk visits every position the teacher faces as X, with the opponent
// replying every legal way.
func walk(t *testing.T, teacher func() agent, visit func(game.Sight)) {
	t.Helper()
	var rules game.RuleSet
	var step func(world msg.TTTWorld)
	step = func(world msg.TTTWorld) {
		if world.Over {
			return
		}
		seat := rules.ActingSlots(&world)[0]
		obs := rules.Project(&world, seat)

		if seat == game.SlotX {
			visit(obs)
			a := teacher()
			a.Observe(obs)
			move, ok := a.Decide(context.Background())
			if !ok {
				t.Fatalf("the teacher passed on its own turn: %v", world.Cells)
			}
			next := clone(world)
			rules.Apply(&next, seat, move)
			step(next)
			return
		}
		for _, cell := range obs.Legal {
			next := clone(world)
			rules.Apply(&next, seat, msg.Move{Cell: uint8(cell)})
			step(next)
		}
	}
	step(rules.Start(0))
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
		case "creates_fork_at":
			holds = pred.CreatesForkAt(obs, cell)
		case "blocks_fork_at":
			holds = pred.BlocksForkAt(obs, cell)
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

func clone(w msg.TTTWorld) msg.TTTWorld {
	c := w
	c.Cells = append([]uint8(nil), w.Cells...)
	c.Line = append([]uint8(nil), w.Line...)
	return c
}

// update rewrites the committed generated sources:
//
//	go test ./tutorial/step5-simulate/distill -update
var update = flag.Bool("update", false, "rewrite the committed generated sources instead of comparing against them")

// TestGeneratedSourcesAreCurrent keeps the committed agent honest.
func TestGeneratedSourcesAreCurrent(t *testing.T) {
	lib, records, err := distill.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	agentSrc, err := behavior.GenerateAgent(distill.Spec(), distill.Judgement(), lib)
	if err != nil {
		t.Fatal(err)
	}
	current(t, "gen/agent_gen.go", agentSrc)

	tests, err := behavior.GenerateTests(distill.TestSpec(), records, 24)
	if err != nil {
		t.Fatal(err)
	}
	current(t, "gen/agent_gen_test.go", tests)

	if *update {
		if err := lib.Save("gen/chips.json"); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := behavior.LoadLibrary("gen/chips.json")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(lib)
	b, _ := json.Marshal(committed)
	if string(a) != string(b) {
		t.Fatal("gen/chips.json is stale; rerun: go test ./tutorial/step5-simulate/distill -update")
	}
}

func current(t *testing.T, path string, generated []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, generated, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(generated) != string(committed) {
		t.Fatalf("%s is stale; rerun: go test ./tutorial/step5-simulate/distill -update", path)
	}
}
