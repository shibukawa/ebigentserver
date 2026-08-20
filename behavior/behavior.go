// Package behavior implements the distillation pipeline of
// flow:behavior-tree-synthesis: recorded decisions are segmented against
// a predicate vocabulary, an analyzer proposes condition→action
// candidates with evidence, a developer approves them into the shared
// chip library (decision:shared-chip-library), and approved chips
// compile to plain Go (decision:behavior-tree-compiled-to-go).
//
// The analyzer is an interface: the concept's analysis step belongs to an
// actor:llm-agent, and SequentialCovering is the deterministic baseline
// implementation that keeps the whole pipeline testable without one. Both
// produce the same artifact shapes, so swapping analyzers never changes
// the review, merge, or codegen stages.
package behavior

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Feature is one data:derived-predicate: a named judgement over the
// observation, with both a runtime evaluator (for mining recorded JSON
// observations) and a Go expression (for generated code over the real
// observation type). The name is the vocabulary term a developer reads.
type Feature struct {
	// Name is the vocabulary term, e.g. "cell_4_empty".
	Name string
	// GoExpr evaluates the predicate over the identifier `obs` in
	// generated code, e.g. "obs.Board[4] == ttt.Empty".
	GoExpr string
	// Eval judges a recorded observation.
	Eval func(obs json.RawMessage) (bool, error)
}

// ActionDef names one action the vocabulary can propose.
type ActionDef struct {
	// Name is the action term, e.g. "play_4".
	Name string
	// GoExpr constructs the action in generated code, e.g.
	// "ttt.Move{Cell: 4}".
	GoExpr string
	// Match reports whether a recorded action is this one.
	Match func(action json.RawMessage) (bool, error)
}

// Vocabulary is the game's predicate and action language
// (rule:analysis-restricted-to-visible-fields holds by construction:
// features only ever see the recorded observation).
type Vocabulary struct {
	Features []Feature
	Actions  []ActionDef
}

// Record is one segmented decision point: which predicates held and
// which action was taken.
type Record struct {
	Episode string
	Tick    uint64
	Slot    uint16
	Obs     json.RawMessage
	// Action is the matched ActionDef name; empty when no definition
	// matched (such records are excluded from mining).
	Action string
	// Bits[i] is Features[i] evaluated on Obs.
	Bits []bool
}

// Evidence is concept:behavior-evidence in reference form: a replayable
// moment in the corpus.
type Evidence struct {
	Episode string `json:"episode"`
	Tick    uint64 `json:"tick"`
}

// Candidate is data:behavior-candidate: one proposed rule with the
// evidence and reasoning that justify it. Condition and Action are
// vocabulary names; decision-list semantics apply (earlier rules take
// precedence, so a condition needs no explicit negations of its
// predecessors).
type Candidate struct {
	Condition       string     `json:"condition"`
	Action          string     `json:"action"`
	Priority        int        `json:"priority"`
	Coverage        int        `json:"coverage"`
	Counterexamples int        `json:"counterexamples"`
	Evidence        []Evidence `json:"evidence,omitempty"`
	Rationale       string     `json:"rationale,omitempty"`
}

// Chip is data:behavior-chip: a candidate that entered the shared
// library. Only a developer's approval turns it into behavior
// (rule:generated-behavior-requires-approval); a rejection is remembered
// so regeneration can say "you rejected this before, because ...".
type Chip struct {
	Condition       string     `json:"condition"`
	Action          string     `json:"action"`
	Priority        int        `json:"priority"`
	Coverage        int        `json:"coverage"`
	Counterexamples int        `json:"counterexamples"`
	Evidence        []Evidence `json:"evidence,omitempty"`
	Rationale       string     `json:"rationale,omitempty"`
	Approved        bool       `json:"approved"`
	Rejected        bool       `json:"rejected,omitempty"`
	RejectReason    string     `json:"reject_reason,omitempty"`
	// Tags carry the open gating dimensions (level, style, tactic) of
	// decision:shared-chip-library.
	Tags []string `json:"tags,omitempty"`
}

// Key identifies a chip across regenerations.
func (c Chip) Key() string { return c.Condition + "→" + c.Action }

// Library is the shared chip library of one game, persisted as JSON.
type Library struct {
	Game  string `json:"game"`
	Chips []Chip `json:"chips"`
}

// Approved returns the active rule set in priority order.
func (l *Library) Approved() []Chip {
	var out []Chip
	for _, c := range l.Chips {
		if c.Approved && !c.Rejected {
			out = append(out, c)
		}
	}
	return out
}

func (l *Library) find(key string) *Chip {
	for i := range l.Chips {
		if l.Chips[i].Key() == key {
			return &l.Chips[i]
		}
	}
	return nil
}

// Save writes the library; Load reads it back.
func (l *Library) Save(path string) error {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// LoadLibrary reads a library file.
func LoadLibrary(path string) (*Library, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l Library
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("behavior: %s: %w", path, err)
	}
	return &l, nil
}

// Featurize evaluates the vocabulary over one decision point.
func (v *Vocabulary) Featurize(episode string, tick uint64, slot uint16, obs, action json.RawMessage) (Record, error) {
	r := Record{Episode: episode, Tick: tick, Slot: slot, Obs: obs, Bits: make([]bool, len(v.Features))}
	for i, f := range v.Features {
		b, err := f.Eval(obs)
		if err != nil {
			return r, fmt.Errorf("behavior: feature %s: %w", f.Name, err)
		}
		r.Bits[i] = b
	}
	for _, a := range v.Actions {
		ok, err := a.Match(action)
		if err != nil {
			return r, fmt.Errorf("behavior: action %s: %w", a.Name, err)
		}
		if ok {
			r.Action = a.Name
			break
		}
	}
	return r, nil
}

// ErrEmptyCorpus reports segmentation that produced nothing to mine.
var ErrEmptyCorpus = errors.New("behavior: no decision records matched the vocabulary")
