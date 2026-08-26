// Package distill turns the recorded play of step 3's bot back into its
// own source code.
//
// The pipeline is the framework's — segment, mine, approve, generate —
// and this package supplies the two things it cannot know: what a corpus
// of this game looks like, and what words a rule about it may use.
//
// It carries two vocabularies on purpose. Naive is the one anybody
// writes first, and Judgement is the one that works. Keeping the loser
// committed and tested is the point of the step: the difference between
// them is not a detail of tic-tac-toe, it is where the leverage in
// distillation actually sits.
package distill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/distill/pred"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/game"
	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/msg"
	"github.com/shibukawa/fixmath"
)

// CorpusMatches is the recipe the committed generated sources came from.
//
// Four hundred games, half what step 4 needed, and the halving is the
// result of the first part of this step rather than a tuning choice: the
// opponent rotates. Step 4 ended on a corpus whose mined ordering was
// correct by how the frequencies happened to fall, and doubling the
// games was what eventually made it fall the right way. Rotating who
// plays gets there at half the volume, because the positions that settle
// an ordering come from opponents that produce them rather than from
// waiting for a coin to.
const CorpusMatches = 400

// sightShape is the part of a recorded sight the miner reads. It decodes
// the JSON the log holds rather than reusing game.Sight, because the
// miner works on files that a different build wrote: the column names in
// data:episode-log are the contract, and a struct that silently followed
// a rename in the game would hide the break instead of reporting it.
type sightShape struct {
	Mark  string   `json:"mark"`
	Cells []string `json:"cells"`
	Legal []int    `json:"legal"`
}

// moveShape is the same for the action side.
type moveShape struct {
	Cell int `json:"cell"`
}

// Naive is the vocabulary of raw field reads: nine judgements of the
// form "cell k holds no mark", and nine placements.
//
// It is what the obvious first attempt looks like, and it is committed
// so that the attempt can be run. Mining against it does not fail — it
// produces a list whose rules carry counterexamples, and a rule with a
// counterexample is never approved, so the agent generated from it is
// silent in exactly the positions the bot handled by knowing something.
func Naive() *behavior.Vocabulary {
	v := &behavior.Vocabulary{}
	for k := 0; k < 9; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("cell_%d_empty", k),
			Doc:    fmt.Sprintf("board cell %d holds no mark", k),
			GoExpr: fmt.Sprintf("obs.Cells[%d] == game.Empty", k),
			Eval: func(raw json.RawMessage) (bool, error) {
				o, err := decodeSight(raw)
				if err != nil {
					return false, err
				}
				return len(o.Cells) == 9 && o.Cells[k] == "-", nil
			},
		})
	}
	addPlacements(v)
	return v
}

// Judgement is the vocabulary that closes: the same nine placements,
// under names for the three distinctions the board does not spell out.
//
// Every feature here compiles to a call into package pred, so the term a
// reviewer reads in a chip and the term the generated agent evaluates are
// the same term. That is the property the naive vocabulary also had and
// which is not what separates them — what separates them is that these
// names describe a judgement about the position rather than a fact about
// one square.
func Judgement() *behavior.Vocabulary {
	v := &behavior.Vocabulary{}
	for k := 0; k < 9; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("winning_move_is_%d", k),
			Doc:    fmt.Sprintf("this seat finishes a line by taking cell %d", k),
			GoExpr: fmt.Sprintf("pred.WinningMoveIs(obs, %d)", k),
			Eval:   evalCompletes(k, own),
		})
	}
	for k := 0; k < 9; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("blocking_move_is_%d", k),
			Doc:    fmt.Sprintf("the opponent finishes a line next turn unless cell %d is taken", k),
			GoExpr: fmt.Sprintf("pred.BlockingMoveIs(obs, %d)", k),
			Eval:   evalCompletes(k, opponent),
		})
	}
	for k := 0; k < 9; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("preferred_cell_is_%d", k),
			Doc:    fmt.Sprintf("with nothing urgent on the board, cell %d is the one to take", k),
			GoExpr: fmt.Sprintf("pred.PreferredCellIs(obs, %d)", k),
			Eval: func(raw json.RawMessage) (bool, error) {
				o, err := decodeSight(raw)
				if err != nil || len(o.Legal) == 0 {
					return false, err
				}
				for _, c := range pred.Preference {
					for _, l := range o.Legal {
						if l == c {
							return c == k, nil
						}
					}
				}
				return false, nil
			},
		})
	}
	addPlacements(v)
	return v
}

// Fork is Judgement plus the two words a perfect player needs.
//
// A player that never loses does not only take wins and stop losses; it
// makes positions with two threats at once, and it denies the opponent
// the same. Neither move is describable in Judgement's words — nothing
// there is about a board that does not exist yet — so a corpus of
// perfect play mined against Judgement leaves those decisions to rules
// that guess.
//
// The fork terms go first because a decision list reads top down and
// these outrank a preference. Where they sit relative to the wins and
// blocks is left to the miner, as it was in step 4.
func Fork() *behavior.Vocabulary {
	v := &behavior.Vocabulary{}
	for k := 0; k < 9; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("creates_fork_at_%d", k),
			Doc:    fmt.Sprintf("taking cell %d leaves this seat with two threats at once", k),
			GoExpr: fmt.Sprintf("pred.CreatesForkAt(obs, %d)", k),
			Eval:   evalFork(k, own),
		})
	}
	for k := 0; k < 9; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("blocks_fork_at_%d", k),
			Doc:    fmt.Sprintf("taking cell %d denies the opponent a fork there", k),
			GoExpr: fmt.Sprintf("pred.BlocksForkAt(obs, %d)", k),
			Eval:   evalFork(k, opponent),
		})
	}
	base := Judgement()
	v.Features = append(v.Features, base.Features...)
	v.Actions = base.Actions
	return v
}

// evalFork is the miner-side twin of pred.CreatesForkAt and
// pred.BlocksForkAt.
func evalFork(cell int, side whose) func(json.RawMessage) (bool, error) {
	return func(raw json.RawMessage) (bool, error) {
		o, err := decodeSight(raw)
		if err != nil || len(o.Legal) == 0 || len(o.Cells) != 9 {
			return false, err
		}
		if cell < 0 || cell >= 9 || o.Cells[cell] != "-" {
			return false, nil
		}
		mark := o.Mark
		if side == opponent {
			if mark == "X" {
				mark = "O"
			} else {
				mark = "X"
			}
		}
		cells := append([]string(nil), o.Cells...)
		cells[cell] = mark

		threats := 0
		for _, line := range game.Lines {
			held, free := 0, 0
			for _, c := range line {
				switch cells[c] {
				case "-":
					free++
				case mark:
					held++
				}
			}
			if held == 2 && free == 1 {
				threats++
			}
		}
		return threats >= 2, nil
	}
}

// whose selects which mark a completion predicate is about.
type whose int

const (
	own whose = iota
	opponent
)

// evalCompletes is the miner-side twin of pred.WinningMoveIs and
// pred.BlockingMoveIs, reading the recorded JSON instead of the live
// sight. The two have to agree, and TestMinerAndRuntimeAgree is what
// keeps them agreeing: a predicate that means one thing while mining and
// another at runtime would produce a library that passes every check and
// an agent that plays differently.
func evalCompletes(cell int, side whose) func(json.RawMessage) (bool, error) {
	return func(raw json.RawMessage) (bool, error) {
		o, err := decodeSight(raw)
		if err != nil || len(o.Legal) == 0 || len(o.Cells) != 9 {
			return false, err
		}
		mark := o.Mark
		if side == opponent {
			if mark == "X" {
				mark = "O"
			} else {
				mark = "X"
			}
		}
		for _, line := range game.Lines {
			var empty int
			held, free := 0, 0
			for _, c := range line {
				switch o.Cells[c] {
				case "-":
					empty, free = int(c), free+1
				case mark:
					held++
				}
			}
			if held == 2 && free == 1 {
				return empty == cell, nil
			}
		}
		return false, nil
	}
}

// addPlacements gives a vocabulary the nine actions.
func addPlacements(v *behavior.Vocabulary) {
	for k := 0; k < 9; k++ {
		v.Actions = append(v.Actions, behavior.ActionDef{
			Name:   fmt.Sprintf("play_%d", k),
			Doc:    fmt.Sprintf("place this seat's mark on cell %d", k),
			GoExpr: fmt.Sprintf("msg.Move{Cell: %d}", k),
			Match: func(raw json.RawMessage) (bool, error) {
				var m moveShape
				if err := json.Unmarshal(raw, &m); err != nil {
					return false, err
				}
				return m.Cell == k, nil
			},
		})
	}
}

func decodeSight(raw json.RawMessage) (sightShape, error) {
	var o sightShape
	err := json.Unmarshal(raw, &o)
	return o, err
}

// Corpus plays the bot against a varying opponent and returns its
// decisions, featurized against v.
//
// The opponent is random on purpose. Two copies of the bot play one game
// however many times you run them — step 3 has a test that says so — and
// a corpus of one game teaches one game. Randomness here is the cheapest
// possible source of the variety step 5 gets properly.
//
// Only the bot's seat is kept. The random opponent's decisions are in the
// same log under a different agent_kind, and distilling those would
// produce a faithful reproduction of a coin.
func Corpus(matches int, v *behavior.Vocabulary) ([]behavior.Record, error) {
	var out []behavior.Record
	for i := 0; i < matches; i++ {
		var decisions bytes.Buffer
		id := fmt.Sprintf("ttt-%03d", i)
		w := episode.NewWriter[msg.TTTWorld, msg.Move, game.Sight](
			episode.Streams{Decisions: &decisions},
			episode.ReplayComplete,
			episode.Meta{
				EpisodeID:  id,
				AgentKinds: map[session.SlotID]string{game.SlotX: "bot", game.SlotO: "random"},
			})

		cfg := game.Config(id, uint64(i))
		cfg.Recorder = w
		// The recorded latency is wall-clock, and a corpus that
		// remembers how fast the machine was on the day is a corpus
		// that cannot be regenerated.
		cfg.Clock = func() int64 { return 0 }

		s, err := session.New(cfg)
		if err != nil {
			return nil, err
		}
		if err := s.OpenAdmission(); err != nil {
			return nil, err
		}
		if err := s.Admit(game.SlotX, &game.Bot{}); err != nil {
			return nil, err
		}
		if err := s.Admit(game.SlotO, newRandom(uint64(i))); err != nil {
			return nil, err
		}
		if err := s.Run(context.Background()); err != nil {
			return nil, err
		}

		recs, err := behavior.Segment(v, "", &decisions, func(row episode.Decision) bool {
			return row.Slot == uint16(game.SlotX)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
	}
	if len(out) == 0 {
		return nil, behavior.ErrEmptyCorpus
	}
	return out, nil
}

// Synthesize records a corpus, mines it, and approves what came back
// clean.
//
// Approving on "no counterexamples" is the whole of the automation, and
// it is deliberately not a judgement about quality. A rule the corpus
// contradicts even once stays out of the library and waits for a person
// (rule:generated-behavior-requires-approval); a rule the corpus never
// contradicts is still only as good as the corpus, which is why the
// generated agent is put back on the board afterwards rather than
// declared correct here.
func Synthesize(matches int, v *behavior.Vocabulary) (*behavior.Library, []behavior.Record, error) {
	records, err := Corpus(matches, v)
	if err != nil {
		return nil, nil, err
	}
	cands, uncovered, err := behavior.SequentialCovering{}.Propose(v, records)
	if err != nil {
		return nil, nil, err
	}
	if len(uncovered) > 0 {
		return nil, nil, fmt.Errorf("distill: %d of %d decisions matched no rule at all",
			len(uncovered), len(records))
	}
	lib := &behavior.Library{Game: "tictactoe"}
	behavior.Merge(lib, cands)
	for i := range lib.Chips {
		if lib.Chips[i].Counterexamples == 0 {
			lib.Chips[i].Approved = true
			lib.Chips[i].Tags = []string{"style:tactic"}
		}
	}
	return lib, records, nil
}

// Covered reports how much of the corpus the approved chips actually
// account for.
//
// It exists because the number that looks like the answer is not one.
// Propose reports uncovered records, and against the naive vocabulary
// that count is zero while more than a third of the decisions are
// explained by rules nobody approved. This is the figure that separates
// the two vocabularies, so it is worth computing rather than inferring.
func Covered(lib *behavior.Library, records []behavior.Record) int {
	n := 0
	for _, chip := range lib.Approved() {
		n += chip.Coverage
	}
	if n > len(records) {
		return len(records)
	}
	return n
}

// Outcome is how one playtest match ended for the seat under test.
type Outcome struct {
	Terminal session.Terminal
	Ticks    session.Tick
}

// Playtest runs one match with the given agent in the first seat and the
// same seeded opponent Corpus uses in the second.
//
// It is the validate step of flow:behavior-tree-synthesis: a generated
// policy is not finished when it compiles, it is finished when it plays.
// Handing it the same seeds the hand-written bot played turns "the same
// policy" from a claim about source code into a claim about matches.
func Playtest(seed uint64, agent session.Agent[game.Sight, msg.Move]) (Outcome, error) {
	cfg := game.Config(fmt.Sprintf("playtest-%d", seed), seed)
	cfg.Clock = func() int64 { return 0 }
	// Watch is the wrapper's own way of keeping each seat's final
	// signal without re-reading the episode it just wrote. Nothing is
	// recorded here, so it wraps nothing.
	watch := run.Watch[msg.TTTWorld, msg.Move, game.Sight](nil)
	cfg.Recorder = watch

	s, err := session.New(cfg)
	if err != nil {
		return Outcome{}, err
	}
	if err := s.OpenAdmission(); err != nil {
		return Outcome{}, err
	}
	if err := s.Admit(game.SlotX, agent); err != nil {
		return Outcome{}, err
	}
	if err := s.Admit(game.SlotO, newRandom(seed)); err != nil {
		return Outcome{}, err
	}
	if err := s.Run(context.Background()); err != nil {
		return Outcome{}, err
	}
	for _, o := range watch.Outcomes() {
		if o.Slot == game.SlotX {
			return Outcome{Terminal: o.Signal.Terminal, Ticks: s.Tick()}, nil
		}
	}
	return Outcome{}, fmt.Errorf("distill: seat X produced no outcome")
}

// Spec targets the generated agent at this game's types.
func Spec() behavior.CodegenSpec {
	return behavior.CodegenSpec{
		Package: "gen",
		Imports: []string{
			"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/distill/pred",
			"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/game",
			"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/msg",
		},
		ObsType:       "game.Sight",
		ActionType:    "msg.Move",
		AgentName:     "Distilled",
		SessionImport: "github.com/shibukawa/ebigentserver/session",
	}
}

// TestSpec is Spec narrowed to what the fixture test actually names.
//
// The generated test decodes a recorded sight and compares action names,
// so the sight type is the only one it mentions: the predicates it never
// calls and the action type it never spells would both be unused imports
// and would not compile. The agent file keeps all three.
func TestSpec() behavior.CodegenSpec {
	s := Spec()
	s.Imports = []string{"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/game"}
	return s
}

// RotatedSpec targets the bot this step actually produces: a different
// package and a different name, so replacing the step-4 bot is a visible
// one-line diff in main.go rather than the same identifier quietly
// meaning something better.
func RotatedSpec() behavior.CodegenSpec {
	s := Spec()
	s.Package = "genrotated"
	s.AgentName = "Rotated"
	return s
}

// RotatedTestSpec is RotatedSpec narrowed the way TestSpec narrows Spec.
func RotatedTestSpec() behavior.CodegenSpec {
	s := RotatedSpec()
	s.Imports = []string{"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/game"}
	return s
}

// randomBot fills the other seat. It picks uniformly among the cells the
// sight says are legal, from a seeded generator: a corpus has to be
// reproducible, so the randomness is a declared input rather than a
// property of when it ran (rule:shared-rng-seed).
type randomBot struct {
	rng  fixmath.Rand
	last game.Sight
}

func newRandom(seed uint64) *randomBot { return &randomBot{rng: fixmath.NewRand(seed | 1)} }

func (*randomBot) Joined(session.SlotID)  {}
func (r *randomBot) Observe(o game.Sight) { r.last = o }
func (*randomBot) Ended(session.Result)   {}

func (r *randomBot) Decide(context.Context) (msg.Move, bool) {
	if len(r.last.Legal) == 0 {
		return msg.Move{}, false
	}
	return msg.Move{Cell: uint8(r.last.Legal[r.rng.Int64n(int64(len(r.last.Legal)))])}, true
}

// Compiled is one distillation's output: the chips it approved and the
// records they were mined from.
//
// It exists so that the command that writes the generated sources and
// the test that checks they are current go through one call. When those
// two each built their own corpus, they could disagree about the recipe
// — and a regeneration loop whose two halves disagree cannot be closed
// by running either of them.
type Compiled struct {
	Library *behavior.Library
	Records []behavior.Record
}

// Compile runs the canonical recipe: the rotated corpus the committed
// genrotated sources came from, and the only one that reproduces them.
func Compile() (*Compiled, error) {
	lib, records, err := Canonical()
	if err != nil {
		return nil, err
	}
	return &Compiled{Library: lib, Records: records}, nil
}

// BaselineMatches is step 4's canonical count, carried into this module
// for the bot the rotation replaces.
const BaselineMatches = 800

// CompileBaseline is step 4's recipe rebuilt here: the same teacher
// against a random opponent alone. It is committed as distill/gen so
// that swapping it out for the rotated bot is a diff a reader can make
// and unmake, with both sides reproducible.
func CompileBaseline() (*Compiled, error) {
	lib, records, err := MineFrom(BaselineMatches, Judgement(), Tactic, RandomOnly())
	if err != nil {
		return nil, err
	}
	return &Compiled{Library: lib, Records: records}, nil
}

// Sources renders every generated file, keyed by its name under the
// output directory.
func (c *Compiled) Sources() (map[string][]byte, error) {
	return c.SourcesAs(Spec(), TestSpec())
}

// SourcesAs renders the same files under the identity the spec carries —
// gen.Distilled for the baseline, genrotated.Rotated for this step's
// product.
func (c *Compiled) SourcesAs(spec, testSpec behavior.CodegenSpec) (map[string][]byte, error) {
	agent, err := behavior.GenerateAgent(spec, Judgement(), c.Library)
	if err != nil {
		return nil, err
	}
	tests, err := behavior.GenerateTests(testSpec, c.Records, 24)
	if err != nil {
		return nil, err
	}
	chips, err := json.MarshalIndent(c.Library, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		"agent_gen.go":      agent,
		"agent_gen_test.go": tests,
		"chips.json":        append(chips, '\n'),
	}, nil
}

// Write puts them on disk. This is what `ebigent distill` reaches
// through ./cmd/distill.
func (c *Compiled) Write(dir string) error {
	return c.write(dir, c.Sources)
}

// WriteAs is Write under another identity, for the rotated bot.
func (c *Compiled) WriteAs(dir string, spec, testSpec behavior.CodegenSpec) error {
	return c.write(dir, func() (map[string][]byte, error) { return c.SourcesAs(spec, testSpec) })
}

func (c *Compiled) write(dir string, render func() (map[string][]byte, error)) error {
	files, err := render()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			return err
		}
	}
	return nil
}
