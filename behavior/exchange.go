package behavior

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// This file is the file-based contract between the pipeline and an
// external analyzer — in practice an LLM working inside the developer's
// own agentic environment (the skills/behavior-analyze skill). The
// pipeline exports an AnalysisRequest; the analyzer writes Proposals;
// ValidateProposals is the trust boundary: vocabulary membership is
// checked mechanically (rule:analysis-restricted-to-visible-fields) and
// every number the analyzer claimed is recomputed from the featurized
// records — a proposal is advice, never authority.

// RequestFeature describes one predicate to the analyzer.
type RequestFeature struct {
	Name   string `json:"name"`
	Doc    string `json:"doc,omitempty"`
	GoExpr string `json:"go_expr"`
}

// RequestAction describes one action to the analyzer.
type RequestAction struct {
	Name   string `json:"name"`
	Doc    string `json:"doc,omitempty"`
	GoExpr string `json:"go_expr"`
}

// RequestRecord is one featurized decision point. Bits is a compact
// string of '0'/'1' per feature (in feature order) so the file stays
// readable and diffable at corpus size.
type RequestRecord struct {
	Episode string `json:"episode"`
	Tick    uint64 `json:"tick"`
	Slot    uint16 `json:"slot"`
	Action  string `json:"action"`
	Bits    string `json:"bits"`
}

// AnalysisRequest is everything the analyzer may use: the vocabulary,
// the featurized corpus, and the current library (so proposals can be
// diff-aware). Sights are deliberately absent by default — the
// bits ARE the visible facts, which enforces the sight boundary at
// the file format level; set IncludeSights for games whose
// analyzer needs the raw view.
type AnalysisRequest struct {
	Game     string           `json:"game"`
	Features []RequestFeature `json:"features"`
	Actions  []RequestAction  `json:"actions"`
	Records  []RequestRecord  `json:"records"`
	Library  *Library         `json:"library,omitempty"`
	// CorpusRoot points at the episode directories for deeper digging
	// (duckdb over the JSONL streams).
	CorpusRoot string `json:"corpus_root,omitempty"`
}

// BuildAnalysisRequest assembles the export.
func BuildAnalysisRequest(game string, v *Vocabulary, records []Record, lib *Library, corpusRoot string) AnalysisRequest {
	req := AnalysisRequest{Game: game, Library: lib, CorpusRoot: corpusRoot}
	for _, f := range v.Features {
		req.Features = append(req.Features, RequestFeature{Name: f.Name, Doc: f.Doc, GoExpr: f.GoExpr})
	}
	for _, a := range v.Actions {
		req.Actions = append(req.Actions, RequestAction{Name: a.Name, Doc: a.Doc, GoExpr: a.GoExpr})
	}
	for _, r := range records {
		bits := make([]byte, len(r.Bits))
		for i, b := range r.Bits {
			bits[i] = '0'
			if b {
				bits[i] = '1'
			}
		}
		req.Records = append(req.Records, RequestRecord{
			Episode: r.Episode, Tick: r.Tick, Slot: r.Slot, Action: r.Action, Bits: string(bits),
		})
	}
	return req
}

// Save writes the request file.
func (r AnalysisRequest) Save(path string) error {
	b, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// PredicateProposal is the analyzer's vocabulary-layer output: a new
// derived predicate it wishes existed. It is a request to a developer —
// the Go body is a draft to review into the game's predicate package,
// never something the pipeline compiles on its own.
type PredicateProposal struct {
	Name      string `json:"name"`
	Doc       string `json:"doc"`
	GoDraft   string `json:"go_draft,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}

// Proposals is what the analyzer writes back.
type Proposals struct {
	Game       string              `json:"game"`
	Candidates []Candidate         `json:"candidates"`
	Predicates []PredicateProposal `json:"predicates,omitempty"`
	// Notes is the analyzer's free-form summary for the reviewer.
	Notes string `json:"notes,omitempty"`
}

// LoadProposals reads a proposals file.
func LoadProposals(path string) (*Proposals, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Proposals
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("behavior: %s: %w", path, err)
	}
	return &p, nil
}

// ValidationIssue is one rejected or corrected claim.
type ValidationIssue struct {
	Candidate string `json:"candidate"`
	Kind      string `json:"kind"` // unknown_condition, unknown_action, coverage_corrected, evidence_invalid, no_coverage
	Detail    string `json:"detail"`
}

// ValidateProposals checks candidates against the request: conditions
// and actions must exist in the vocabulary, and coverage,
// counterexamples, and evidence are recomputed from the records under
// decision-list semantics (candidates evaluated in their given order).
// The returned candidates carry only recomputed numbers; claimed ones
// are discarded. Candidates naming unknown vocabulary are dropped with
// an issue — the mechanical form of
// rule:analysis-restricted-to-visible-fields.
func ValidateProposals(req AnalysisRequest, p *Proposals) (valid []Candidate, issues []ValidationIssue) {
	featIdx := map[string]int{}
	for i, f := range req.Features {
		featIdx[f.Name] = i
	}
	actions := map[string]bool{}
	for _, a := range req.Actions {
		actions[a.Name] = true
	}
	type rec struct {
		RequestRecord
		taken bool
	}
	records := make([]rec, len(req.Records))
	for i, r := range req.Records {
		records[i] = rec{RequestRecord: r}
	}
	momentOK := map[string]bool{}
	for _, r := range req.Records {
		momentOK[fmt.Sprintf("%s@%d", r.Episode, r.Tick)] = true
	}

	for _, cand := range p.Candidates {
		key := cand.Condition + "→" + cand.Action
		fi, ok := featIdx[cand.Condition]
		if !ok {
			issues = append(issues, ValidationIssue{key, "unknown_condition",
				fmt.Sprintf("predicate %q is not in the vocabulary; the runtime agent could never evaluate it", cand.Condition)})
			continue
		}
		if !actions[cand.Action] {
			issues = append(issues, ValidationIssue{key, "unknown_action",
				fmt.Sprintf("action %q is not in the vocabulary", cand.Action)})
			continue
		}
		// Recompute at this decision-list position. Matching is
		// tentative: a rule dropped for zero coverage does not exist in
		// the final list, so it must not consume records from the rules
		// after it.
		coverage, counter := 0, 0
		var evidence []Evidence
		var matched []int
		for i := range records {
			r := &records[i]
			if r.taken || fi >= len(r.Bits) || r.Bits[fi] != '1' {
				continue
			}
			matched = append(matched, i)
			if r.Action == cand.Action {
				coverage++
				if len(evidence) < 5 {
					evidence = append(evidence, Evidence{Episode: r.Episode, Tick: r.Tick})
				}
			} else {
				counter++
			}
		}
		if coverage == 0 {
			issues = append(issues, ValidationIssue{key, "no_coverage",
				"no remaining record supports this rule at its list position"})
			continue
		}
		for _, i := range matched {
			records[i].taken = true // an accepted rule handles these from here on
		}
		if cand.Coverage != 0 && (cand.Coverage != coverage || cand.Counterexamples != counter) {
			issues = append(issues, ValidationIssue{key, "coverage_corrected",
				fmt.Sprintf("claimed %d/%d, recomputed %d/%d", cand.Coverage, cand.Counterexamples, coverage, counter)})
		}
		for _, ev := range cand.Evidence {
			if !momentOK[fmt.Sprintf("%s@%d", ev.Episode, ev.Tick)] {
				issues = append(issues, ValidationIssue{key, "evidence_invalid",
					fmt.Sprintf("cited moment %s@%d is not in the corpus", ev.Episode, ev.Tick)})
			}
		}
		valid = append(valid, Candidate{
			Condition: cand.Condition, Action: cand.Action, Priority: len(valid),
			Coverage: coverage, Counterexamples: counter,
			Evidence: evidence, Rationale: cand.Rationale,
		})
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Candidate < issues[j].Candidate })
	return valid, issues
}
