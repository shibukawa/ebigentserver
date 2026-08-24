// Package distill closes the loop the solo example exists to
// demonstrate: enemies that played are read back out of
// data:episode-log, mined into data:behavior-chip, and compiled to Go
// that plays the same way.
//
// Nothing here is engine-adjacent and nothing here is a server. It is the
// payoff of having put a solo game's enemies on concept:player-slot: they
// left a record, and a record can be learned from.
//
// The vocabulary below is shared by both enemy kinds on purpose. The
// predicates are facts about an sight — where the quarry lies, and
// on which axis it is further away — and each kind maps those same facts
// to different moves. That the mined decision lists come out different is
// the proof that the vocabulary is not quietly the policy.
package distill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/examples/solo/game"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// obsShape is the part of a recorded sight the miner reads. The
// coordinates are the raw fixed-point bits, which is what the log holds
// and what the simulation compared (rule:analysis-restricted-to-visible-
// fields: nothing outside the sight is reachable from here).
type obsShape struct {
	Self struct {
		X int64 `json:"x"`
		Y int64 `json:"y"`
	} `json:"self"`
	Quarry struct {
		X int64 `json:"x"`
		Y int64 `json:"y"`
	} `json:"quarry"`
}

// gaps returns the recorded vector from the observer to the quarry.
func gaps(raw json.RawMessage) (dx, dy int64, err error) {
	var o obsShape
	if err := json.Unmarshal(raw, &o); err != nil {
		return 0, 0, err
	}
	return o.Quarry.X - o.Self.X, o.Quarry.Y - o.Self.Y, nil
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// axis names the two ways a gap can be described.
type axis struct {
	// name is the vocabulary suffix.
	name string
	// doc completes the sentence "the quarry is ... ".
	doc string
	// wide selects the axis carrying the larger gap; narrow the
	// smaller. The two overlap when the gaps are equal, which is
	// deliberate: a decision list orders overlapping rules rather than
	// requiring them to be disjoint.
	wide bool
}

// direction names one of the four ways the quarry can lie.
type direction struct {
	name string
	doc  string
	// horizontal selects which gap the direction reads; positive is the
	// sign it looks for. Nothing here names an action: pairing a fact
	// with a move is what mining is for.
	horizontal bool
	positive   bool
}

// Vocabulary is the shared predicate and action language of solo's
// pursuers: where the quarry lies, on which axis, and the five moves.
func Vocabulary() *behavior.Vocabulary {
	v := &behavior.Vocabulary{}

	dirs := []direction{
		{name: "right", doc: "to the right", horizontal: true, positive: true},
		{name: "left", doc: "to the left", horizontal: true, positive: false},
		{name: "below", doc: "below", horizontal: false, positive: true},
		{name: "above", doc: "above", horizontal: false, positive: false},
	}
	axes := []axis{
		{name: "wide_axis", doc: "and further away on that axis than on the other", wide: true},
		{name: "narrow_axis", doc: "and closer on that axis than on the other", wide: false},
	}

	for _, a := range axes {
		for _, d := range dirs {
			v.Features = append(v.Features, feature(d, a))
		}
	}
	v.Features = append(v.Features, behavior.Feature{
		Name:   "quarry_reached",
		Doc:    "the quarry is exactly where the observer is: no gap on either axis",
		GoExpr: "game.GapX(obs) == 0 && game.GapY(obs) == 0",
		Eval: func(raw json.RawMessage) (bool, error) {
			dx, dy, err := gaps(raw)
			return dx == 0 && dy == 0, err
		},
	})

	for _, a := range []struct {
		name, doc, expr string
		move            game.Dir
	}{
		{"move_stay", "hold position this tick", "game.Action{Move: game.Stay}", game.Stay},
		{"move_up", "move one step up", "game.Action{Move: game.Up}", game.Up},
		{"move_down", "move one step down", "game.Action{Move: game.Down}", game.Down},
		{"move_left", "move one step left", "game.Action{Move: game.Left}", game.Left},
		{"move_right", "move one step right", "game.Action{Move: game.Right}", game.Right},
	} {
		move := a.move
		v.Actions = append(v.Actions, behavior.ActionDef{
			Name:   a.name,
			Doc:    a.doc,
			GoExpr: a.expr,
			Match: func(raw json.RawMessage) (bool, error) {
				var act struct {
					Move game.Dir `json:"move"`
				}
				if err := json.Unmarshal(raw, &act); err != nil {
					return false, err
				}
				return act.Move == move, nil
			},
		})
	}
	return v
}

// feature builds one directional judgement: the quarry lies this way, and
// this axis is the wider or narrower of the two.
func feature(d direction, a axis) behavior.Feature {
	gap, other := "game.GapX(obs)", "game.GapY(obs)"
	if !d.horizontal {
		gap, other = other, gap
	}
	sign := ">"
	if !d.positive {
		sign = "<"
	}
	// A tie between the two gaps reads as horizontal, so that "wide" has
	// exactly one meaning when the quarry sits on the diagonal.
	cmp := "<="
	if a.wide {
		cmp = ">="
		if !d.horizontal {
			cmp = ">"
		}
	}

	return behavior.Feature{
		Name:   fmt.Sprintf("quarry_%s_on_%s", d.name, a.name),
		Doc:    fmt.Sprintf("the quarry is %s the observer %s", d.doc, a.doc),
		GoExpr: fmt.Sprintf("%s %s 0 && %s.Abs() %s %s.Abs()", gap, sign, gap, cmp, other),
		Eval: func(raw json.RawMessage) (bool, error) {
			dx, dy, err := gaps(raw)
			if err != nil {
				return false, err
			}
			mine, others := dx, dy
			if !d.horizontal {
				mine, others = dy, dx
			}
			if d.positive && mine <= 0 {
				return false, nil
			}
			if !d.positive && mine >= 0 {
				return false, nil
			}
			switch {
			case a.wide && d.horizontal:
				return abs(mine) >= abs(others), nil
			case a.wide:
				return abs(mine) > abs(others), nil
			default:
				return abs(mine) <= abs(others), nil
			}
		},
	}
}

// Records reads a recorded corpus and segments one seat's decisions into
// featurized data:behavior-evidence. It reads the files on disk rather
// than a buffer kept from the run, so what is mined is exactly what was
// written.
func Records(root string, slot session.SlotID) ([]behavior.Record, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("distill: read corpus: %w", err)
	}
	vocab := Vocabulary()
	var out []behavior.Record
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		f, err := os.Open(filepath.Join(root, e.Name(), "decisions.jsonl"))
		if err != nil {
			continue // not an episode directory
		}
		recs, err := behavior.Segment(vocab, "", f, func(s uint16) bool {
			return session.SlotID(s) == slot
		})
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("distill: %s: %w", e.Name(), err)
		}
		out = append(out, recs...)
	}
	if len(out) == 0 {
		return nil, behavior.ErrEmptyCorpus
	}
	return out, nil
}

// Synthesize mines a decision list and approves the clean rules.
//
// Approval is programmatic here — only rules with no counterexample pass
// — which stands in for a developer's review until they do it themselves.
// The gate of rule:generated-behavior-requires-approval is still a gate;
// this is a script holding it.
func Synthesize(name string, records []behavior.Record) (*behavior.Library, error) {
	cands, uncovered, err := behavior.SequentialCovering{}.Propose(Vocabulary(), records)
	if err != nil {
		return nil, err
	}
	if len(uncovered) > 0 {
		return nil, fmt.Errorf("distill: %d of %d decisions were not covered by any rule",
			len(uncovered), len(records))
	}
	lib := &behavior.Library{Game: "solo"}
	behavior.Merge(lib, cands)
	for i := range lib.Chips {
		if lib.Chips[i].Counterexamples == 0 {
			lib.Chips[i].Approved = true
			lib.Chips[i].Tags = []string{"kind:" + name}
		}
	}
	return lib, nil
}

// Spec targets the generated agent at this game's types. One package per
// kind, because two compiled decision lists in one package would collide
// on the Decide function each of them is.
func Spec(pkg, agentName string) behavior.CodegenSpec {
	return behavior.CodegenSpec{
		Package:       pkg,
		Imports:       []string{"github.com/shibukawa/ebigentserver/examples/solo/game"},
		ObsType:       "game.Sight",
		ActionType:    "game.Action",
		AgentName:     agentName,
		SessionImport: "github.com/shibukawa/ebigentserver/session",
	}
}

// Kind is one distillation target: an enemy seat, the name it carries in
// the episode header, and where its compiled form is written.
type Kind struct {
	// Name is the enemy kind, matching what game.NewAgent labels the
	// seat with.
	Name string
	// Slot is the seat that kind occupied while the corpus was
	// recorded.
	Slot session.SlotID
	// Package and Agent name the generated package and type.
	Package, Agent string
}

// Kinds are the enemies this example distils. Both are mined from the
// same corpus with the same vocabulary, and the decision lists come out
// different — which is the point.
func Kinds() []Kind {
	return []Kind{
		{Name: game.KindChaser, Slot: game.Enemy1, Package: "chaser", Agent: "Chaser"},
		{Name: game.KindFlanker, Slot: game.Enemy2, Package: "flanker", Agent: "Flanker"},
	}
}

// CorpusMatches and CorpusSeed are the recipe the committed generated
// sources under gen/ came from, and the only recipe that reproduces them.
//
// They live here rather than in either caller because there are two, and
// they have to agree: solo-distill writes the files and the test
// compares against them. When those two disagreed, the command named in
// the staleness message wrote output that same message then rejected —
// a loop that cannot close, and one nothing detects, because each half
// is individually consistent.
const (
	CorpusMatches = 16
	CorpusSeed    = 1
)

// Play records matches into root — the corpus every later step reads.
// Every seat is an agent, so this is the same unattended run solo-sim
// performs, called from a library instead of a command.
func Play(ctx context.Context, root string, matches int, seed uint64) error {
	return run.Serve(ctx, game.Options(), game.Binding(), run.ServeOptions{
		Matches: matches,
		Seed:    seed,
		Time:    session.Unlimited,
		Record:  run.RecordOptions{Root: root, Mode: episode.ReplayComplete},
	})
}

// Compiled is one kind's distillation result: the chips, the agent
// source, and the fixture tests that pin it to real recorded moments.
type Compiled struct {
	Kind    Kind
	Library *behavior.Library
	Records []behavior.Record
	Agent   []byte
	Tests   []byte
}

// Write puts one kind's output into dir, the package directory under
// gen/.
//
// It is the only writer of those files. solo-distill calls it to
// regenerate them and the staleness test calls it to produce what it
// compares against, so a file added here is covered by both without
// either being told about it.
func (c *Compiled) Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "agent_gen.go"), c.Agent, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "fixtures_gen_test.go"), c.Tests, 0o644); err != nil {
		return err
	}
	return c.Library.Save(filepath.Join(dir, "chips.json"))
}

// Compile runs the whole pipeline for one kind over a recorded corpus:
// segment, mine, approve, generate.
func Compile(root string, k Kind) (*Compiled, error) {
	records, err := Records(root, k.Slot)
	if err != nil {
		return nil, err
	}
	lib, err := Synthesize(k.Name, records)
	if err != nil {
		return nil, err
	}
	spec := Spec(k.Package, k.Agent)
	agent, err := behavior.GenerateAgent(spec, Vocabulary(), lib)
	if err != nil {
		return nil, err
	}
	tests, err := behavior.GenerateTests(spec, records, 24)
	if err != nil {
		return nil, err
	}
	return &Compiled{Kind: k, Library: lib, Records: records, Agent: agent, Tests: tests}, nil
}
