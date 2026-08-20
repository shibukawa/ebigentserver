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
			Game: ttt.Game{}, Validator: ttt.Validator{},
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
