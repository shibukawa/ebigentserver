// Package distill wires reversi into the behavior pipeline. The point of
// doing it twice (after tic-tac-toe) is the vocabulary: reversi's
// predicates are judgements, not coordinates. "best_move_is_19" names the
// greedy argmax over the observation's legal-move affordances — a
// data:derived-predicate whose body lives in the reviewed dpred package —
// so the mined chips read as strategy ("when the best greedy move is d3,
// play d3") instead of board arithmetic, and the generated decision list
// calls the same reviewed functions the miner judged recordings with.
package distill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/reversi/distill/dpred"
	"github.com/shibukawa/ebigentserver/samples/reversi/reversi"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// Vocabulary is reversi's predicate and action language: 64 best-move
// judgements plus the forced pass, and the 64 placements plus the pass.
// Each feature's Eval judges the recorded observation JSON with the same
// dpred function its GoExpr compiles to, so miner and generated agent
// cannot disagree about what a term means.
func Vocabulary() *behavior.Vocabulary {
	v := &behavior.Vocabulary{}
	// The recorded observation serializes reversi.Observation directly:
	// Legal keeps its Go field name, while LegalMove/Move carry their
	// json tags (move/flips, cell/pass). Reusing reversi.LegalMove for
	// the shape keeps the miner honest about that encoding.
	type obsShape struct {
		Legal []reversi.LegalMove
	}
	type moveShape struct {
		Cell uint8 `json:"cell"`
		Pass bool  `json:"pass"`
	}
	legal := func(raw json.RawMessage) (reversi.Observation, error) {
		var o obsShape
		if err := json.Unmarshal(raw, &o); err != nil {
			return reversi.Observation{}, err
		}
		return reversi.Observation{Legal: o.Legal}, nil
	}
	for k := uint8(0); k < 64; k++ {
		v.Features = append(v.Features, behavior.Feature{
			Name:   fmt.Sprintf("best_move_is_%d", k),
			GoExpr: fmt.Sprintf("dpred.BestMoveIs(obs, %d)", k),
			Eval: func(raw json.RawMessage) (bool, error) {
				obs, err := legal(raw)
				if err != nil {
					return false, err
				}
				return dpred.BestMoveIs(obs, k), nil
			},
		})
		v.Actions = append(v.Actions, behavior.ActionDef{
			Name:   fmt.Sprintf("play_%d", k),
			GoExpr: fmt.Sprintf("reversi.Move{Cell: %d}", k),
			Match: func(raw json.RawMessage) (bool, error) {
				var m moveShape
				if err := json.Unmarshal(raw, &m); err != nil {
					return false, err
				}
				return !m.Pass && m.Cell == k, nil
			},
		})
	}
	v.Features = append(v.Features, behavior.Feature{
		Name:   "must_pass",
		GoExpr: "dpred.MustPass(obs)",
		Eval: func(raw json.RawMessage) (bool, error) {
			obs, err := legal(raw)
			if err != nil {
				return false, err
			}
			return dpred.MustPass(obs), nil
		},
	})
	v.Actions = append(v.Actions, behavior.ActionDef{
		Name:   "pass",
		GoExpr: "reversi.Move{Pass: true}",
		Match: func(raw json.RawMessage) (bool, error) {
			var m moveShape
			if err := json.Unmarshal(raw, &m); err != nil {
				return false, err
			}
			return m.Pass, nil
		},
	})
	return v
}

// randomAgent plays a seeded random legal move (forced pass included,
// since it arrives as the single Legal entry): the varied opponent that
// spreads the greedy bot's decisions across many positions.
type randomAgent struct {
	rng  fixmath.Rand
	last reversi.Observation
}

func newRandomAgent(seed uint64) *randomAgent {
	return &randomAgent{rng: fixmath.NewRand(seed | 1)}
}

func (*randomAgent) Joined(session.SlotID)           {}
func (a *randomAgent) Observe(o reversi.Observation) { a.last = o }
func (*randomAgent) Ended(session.Result)            {}

func (a *randomAgent) Decide(context.Context) (reversi.Move, bool) {
	if len(a.last.Legal) == 0 {
		return reversi.Move{}, false
	}
	return a.last.Legal[a.rng.Int64n(int64(len(a.last.Legal)))].Move, true
}

// NewRandomOpponent exposes the seeded random-legal player for
// playtests.
func NewRandomOpponent(seed uint64) session.Agent[reversi.Observation, reversi.Move] {
	return newRandomAgent(seed)
}

// Corpus plays n matches of GreedyBot (Black) against seeded random
// opponents and returns Black's segmented decisions — the distillation
// input. Fresh seed per match, as concept:continuous-match-loop demands:
// a fixed seed would make every episode a duplicate.
func Corpus(n int) ([]behavior.Record, error) {
	vocab := Vocabulary()
	var records []behavior.Record
	for i := 0; i < n; i++ {
		var decisions bytes.Buffer
		w := episode.NewWriter[reversi.State, reversi.Move, reversi.Observation](
			episode.Streams{Decisions: &decisions},
			episode.ReplayComplete,
			episode.Meta{EpisodeID: fmt.Sprintf("reversi-%03d", i),
				AgentKinds: map[session.SlotID]string{reversi.SlotBlack: "greedy", reversi.SlotWhite: "random"}},
		)
		s, err := session.New(session.Config[reversi.State, reversi.Move, reversi.Observation]{
			ID: fmt.Sprintf("reversi-%03d", i), Slots: reversi.Slots(),
			Game: reversi.Game{}, Validator: reversi.Validator{},
			Recorder: w, Seed: uint64(i)*2654435761 + 1,
			Clock: func() int64 { return 0 },
		})
		if err != nil {
			return nil, err
		}
		if err := s.OpenAdmission(); err != nil {
			return nil, err
		}
		if err := s.Admit(reversi.SlotBlack, &reversi.GreedyBot{}); err != nil {
			return nil, err
		}
		if err := s.Admit(reversi.SlotWhite, newRandomAgent(uint64(i))); err != nil {
			return nil, err
		}
		if err := s.Run(context.Background()); err != nil {
			return nil, err
		}
		recs, err := behavior.Segment(vocab, "", &decisions, func(slot uint16) bool {
			return slot == uint16(reversi.SlotBlack) // distill the greedy seat only
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
// gatekeeper being a script). Not every cell need produce a chip: only
// cells the corpus ever saw as the greedy best appear, and that is the
// library telling the truth about its evidence.
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
	lib := &behavior.Library{Game: "reversi"}
	behavior.Merge(lib, cands)
	for i := range lib.Chips {
		if lib.Chips[i].Counterexamples == 0 {
			lib.Chips[i].Approved = true
			lib.Chips[i].Tags = []string{"level:intermediate", "style:greedy"}
		}
	}
	return lib, records, nil
}

// Spec is the reversi codegen target: the generated agent needs both the
// game types and the dpred judgement package its conditions call.
func Spec() behavior.CodegenSpec {
	return behavior.CodegenSpec{
		Package: "gen",
		Imports: []string{
			"github.com/shibukawa/ebigentserver/samples/reversi/distill/dpred",
			"github.com/shibukawa/ebigentserver/samples/reversi/reversi",
		},
		ObsType:       "reversi.Observation",
		ActionType:    "reversi.Move",
		AgentName:     "DistilledGreedy",
		SessionImport: "github.com/shibukawa/ebigentserver/session",
	}
}

// TestSpec is Spec narrowed for behavior.GenerateTests: the generated
// fixture test only ever names the observation type, so importing dpred
// there would be an unused import and fail the build. The agent file
// keeps both imports; the test file keeps only what it references.
func TestSpec() behavior.CodegenSpec {
	s := Spec()
	s.Imports = []string{"github.com/shibukawa/ebigentserver/samples/reversi/reversi"}
	return s
}
