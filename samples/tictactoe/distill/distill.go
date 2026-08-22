// Package distill wires tic-tac-toe into the behavior pipeline: the
// vocabulary (data:derived-predicate definitions with both a miner-side
// evaluator and a codegen-side Go expression), corpus generation, and the
// synthesis run that turns the first-empty bot's recorded play back into
// its own source code. The smallest possible proof that the
// episode→chip→Go loop closes.
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
	"github.com/shibukawa/ebigentserver/samples/tictactoe/ttt"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// Vocabulary is tic-tac-toe's predicate and action language: nine cell
// emptiness judgements and nine placements. Deliberately naive — the
// point is the pipeline, and even this vocabulary reconstructs the
// sample bot exactly.
func Vocabulary() *behavior.Vocabulary {
	v := &behavior.Vocabulary{}
	type obsShape struct {
		Board [9]uint8
	}
	type moveShape struct {
		Cell uint8
	}
	for k := uint8(0); k < 9; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("cell_%d_empty", k),
			Doc:    fmt.Sprintf("board cell %d (row-major, 0=top-left) holds no mark", k),
			GoExpr: fmt.Sprintf("obs.Board[%d] == ttt.Empty", k),
			Eval: func(raw json.RawMessage) (bool, error) {
				var o obsShape
				if err := json.Unmarshal(raw, &o); err != nil {
					return false, err
				}
				return o.Board[k] == uint8(ttt.Empty), nil
			},
		})
		v.Actions = append(v.Actions, behavior.ActionDef{
			Name:   fmt.Sprintf("play_%d", k),
			Doc:    fmt.Sprintf("place the own mark on cell %d", k),
			GoExpr: fmt.Sprintf("ttt.Move{Cell: %d}", k),
			Match: func(raw json.RawMessage) (bool, error) {
				var m moveShape
				if err := json.Unmarshal(raw, &m); err != nil {
					return false, err
				}
				return m.Cell == k, nil
			},
		})
	}
	return v
}

// randomAgent plays a seeded random legal move: the varied opponent that
// spreads the bot's decisions across many boards.
type randomAgent struct {
	rng  fixmath.Rand
	last ttt.Observation
}

func newRandomAgent(seed uint64) *randomAgent {
	return &randomAgent{rng: fixmath.NewRand(seed | 1)}
}

func (*randomAgent) Joined(session.SlotID)       {}
func (a *randomAgent) Observe(o ttt.Observation) { a.last = o }
func (*randomAgent) Ended(session.Result)        {}

func (a *randomAgent) Decide(context.Context) (ttt.Move, bool) {
	var empty []uint8
	for c := uint8(0); c < 9; c++ {
		if a.last.Board[c] == ttt.Empty {
			empty = append(empty, c)
		}
	}
	if len(empty) == 0 {
		return ttt.Move{}, false
	}
	return ttt.Move{Cell: empty[a.rng.Int64n(int64(len(empty)))]}, true
}

// NewRandomOpponent exposes the seeded random-legal player for
// playtests.
func NewRandomOpponent(seed uint64) session.Agent[ttt.Observation, ttt.Move] {
	return newRandomAgent(seed)
}

// Corpus plays n matches of the sample bot (X) against seeded random
// opponents and returns X's segmented decisions — the distillation
// input. Fresh seed per match, as concept:continuous-match-loop demands:
// a fixed seed would make every episode a duplicate.
func Corpus(n int) ([]behavior.Record, error) {
	vocab := Vocabulary()
	var records []behavior.Record
	for i := 0; i < n; i++ {
		var decisions bytes.Buffer
		w := episode.NewWriter[ttt.State, ttt.Move, ttt.Observation](
			episode.Streams{Decisions: &decisions},
			episode.ReplayComplete,
			episode.Meta{EpisodeID: fmt.Sprintf("ttt-%03d", i),
				AgentKinds: map[session.SlotID]string{ttt.SlotX: "bot", ttt.SlotO: "random"}},
		)
		s, err := session.New(session.Config[ttt.State, ttt.Move, ttt.Observation]{
			ID: fmt.Sprintf("ttt-%03d", i), Slots: ttt.Slots(),
			Simulation: ttt.Simulation{}, Validator: ttt.Validator{},
			Recorder: w, Seed: uint64(i)*2654435761 + 1,
			Clock: func() int64 { return 0 },
		})
		if err != nil {
			return nil, err
		}
		if err := s.OpenAdmission(); err != nil {
			return nil, err
		}
		if err := s.Admit(ttt.SlotX, &ttt.Bot{}); err != nil {
			return nil, err
		}
		if err := s.Admit(ttt.SlotO, newRandomAgent(uint64(i))); err != nil {
			return nil, err
		}
		if err := s.Run(context.Background()); err != nil {
			return nil, err
		}
		recs, err := behavior.Segment(vocab, "", &decisions, func(slot uint16) bool {
			return slot == uint16(ttt.SlotX) // distill the bot's seat only
		})
		if err != nil {
			return nil, err
		}
		records = append(records, recs...)
	}
	if len(records) == 0 {
		return nil, behavior.ErrEmptyCorpus
	}
	return records, nil
}

// Synthesize runs the full pipeline over a fresh corpus and returns the
// approved library plus the corpus it was mined from. Approval here is
// programmatic — only clean rules (zero counterexamples) pass, standing
// in for the developer's review until the editor UI exists
// (rule:generated-behavior-requires-approval keeps the gate; this is the
// gatekeeper being a script).
func Synthesize(matches int) (*behavior.Library, []behavior.Record, error) {
	records, err := Corpus(matches)
	if err != nil {
		return nil, nil, err
	}
	cands, uncovered, err := behavior.SequentialCovering{}.Propose(Vocabulary(), records)
	if err != nil {
		return nil, nil, err
	}
	if len(uncovered) > 0 {
		return nil, nil, fmt.Errorf("distill: %d decisions uncovered", len(uncovered))
	}
	lib := &behavior.Library{Game: "tictactoe"}
	behavior.Merge(lib, cands)
	for i := range lib.Chips {
		if lib.Chips[i].Counterexamples == 0 {
			lib.Chips[i].Approved = true
			lib.Chips[i].Tags = []string{"level:beginner", "style:leftmost"}
		}
	}
	return lib, records, nil
}

// ExportCorpus plays n bot-vs-random matches recording full episode
// directories under root (the shape analysis and the behavior-analyze
// skill read), then returns the bot's featurized records segmented back
// from those files — so the exported analysis request describes exactly
// what is on disk.
func ExportCorpus(root string, n int) ([]behavior.Record, error) {
	vocab := Vocabulary()
	var records []behavior.Record
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("ttt-%03d", i)
		dir := filepath.Join(root, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		files := make([]*os.File, 0, 4)
		open := func(name string) (*os.File, error) {
			f, err := os.Create(filepath.Join(dir, name))
			if err == nil {
				files = append(files, f)
			}
			return f, err
		}
		var streams episode.Streams
		var err error
		if streams.Decisions, err = open("decisions.jsonl"); err == nil {
			if streams.Events, err = open("events.jsonl"); err == nil {
				if streams.Outcomes, err = open("outcomes.jsonl"); err == nil {
					streams.World, err = open("world.jsonl")
				}
			}
		}
		if err != nil {
			return nil, err
		}
		w := episode.NewWriter[ttt.State, ttt.Move, ttt.Observation](
			streams, episode.ReplayComplete,
			episode.Meta{EpisodeID: id,
				AgentKinds: map[session.SlotID]string{ttt.SlotX: "bot", ttt.SlotO: "random"}},
		)
		if err := playMatch(i, w); err != nil {
			return nil, err
		}
		for _, f := range files {
			if err := f.Close(); err != nil {
				return nil, err
			}
		}
		df, err := os.Open(filepath.Join(dir, "decisions.jsonl"))
		if err != nil {
			return nil, err
		}
		recs, err := behavior.Segment(vocab, id, df, func(slot uint16) bool {
			return slot == uint16(ttt.SlotX)
		})
		df.Close()
		if err != nil {
			return nil, err
		}
		records = append(records, recs...)
	}
	return records, nil
}

// playMatch runs one recorded bot-vs-random match.
func playMatch(i int, w *episode.Writer[ttt.State, ttt.Move, ttt.Observation]) error {
	s, err := session.New(session.Config[ttt.State, ttt.Move, ttt.Observation]{
		ID: fmt.Sprintf("ttt-%03d", i), Slots: ttt.Slots(),
		Simulation: ttt.Simulation{}, Validator: ttt.Validator{},
		Recorder: w, Seed: uint64(i)*2654435761 + 1,
		Clock: func() int64 { return 0 },
	})
	if err != nil {
		return err
	}
	if err := s.OpenAdmission(); err != nil {
		return err
	}
	if err := s.Admit(ttt.SlotX, &ttt.Bot{}); err != nil {
		return err
	}
	if err := s.Admit(ttt.SlotO, newRandomAgent(uint64(i))); err != nil {
		return err
	}
	return s.Run(context.Background())
}

// CenterFirstLoadout assembles a different personality from the same
// chip library (decision:shared-chip-library): a tactic that claims the
// center while it is open, falling back to the full leftmost list. Same
// chips, different loadout, different play — no new analysis needed.
func CenterFirstLoadout() *behavior.Loadout {
	return &behavior.Loadout{
		Name: "center-first",
		Tactics: []behavior.Tactic{
			{
				Name:      "claim_center",
				Condition: "cell_4_empty",
				ChipKeys:  []string{"cell_4_empty→play_4"},
			},
			{
				Name: "leftmost",
				ChipKeys: []string{
					"cell_0_empty→play_0", "cell_1_empty→play_1", "cell_2_empty→play_2",
					"cell_3_empty→play_3", "cell_5_empty→play_5", "cell_6_empty→play_6",
					"cell_7_empty→play_7", "cell_8_empty→play_8",
				},
			},
		},
		Profile: map[string]string{"style": "center-first"},
	}
}

// Spec is the tic-tac-toe codegen target.
func Spec() behavior.CodegenSpec {
	return behavior.CodegenSpec{
		Package:       "gen",
		Imports:       []string{"github.com/shibukawa/ebigentserver/samples/tictactoe/ttt"},
		ObsType:       "ttt.Observation",
		ActionType:    "ttt.Move",
		AgentName:     "DistilledAgent",
		SessionImport: "github.com/shibukawa/ebigentserver/session",
	}
}
