package behavior

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/shibukawa/ebigentserver/episode"
)

// Segment reads one episode's decisions stream and featurizes the rows
// the filter accepts (the segment step of flow:behavior-tree-synthesis).
// Sight-only rows and actions outside the vocabulary are skipped.
//
// The filter sees the whole data:decision-record row, not just the seat:
// the agent_kind column is the one that separates a human's rows from a
// bot's within one seat, and a filter that could not read it would make
// that column decoration (requirement:corpus-curation).
func Segment(v *Vocabulary, episodeID string, decisions io.Reader, keep func(row episode.Decision) bool) ([]Record, error) {
	sc := bufio.NewScanner(decisions)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	if !sc.Scan() {
		return nil, sc.Err() // empty stream: no records
	}
	var header episode.Header
	if err := json.Unmarshal(sc.Bytes(), &header); err != nil {
		return nil, fmt.Errorf("behavior: header: %w", err)
	}
	if episodeID == "" {
		episodeID = header.EpisodeID
	}
	var out []Record
	for line := 2; sc.Scan(); line++ {
		var row episode.Decision
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("behavior: decisions line %d: %w", line, err)
		}
		if len(row.Action) == 0 || string(row.Action) == "null" {
			continue
		}
		if keep != nil && !keep(row) {
			continue
		}
		rec, err := v.Featurize(episodeID, row.Tick, row.Slot, row.Sight, row.Action)
		if err != nil {
			return nil, err
		}
		if rec.Action == "" {
			continue // action outside the vocabulary
		}
		out = append(out, rec)
	}
	return out, sc.Err()
}

// Analyzer proposes candidates from segmented decisions. The production
// analyzer is an actor:llm-agent finding situation→action patterns; the
// deterministic baseline below serves tests and simple policies. Both
// speak in the same artifacts, so the rest of the pipeline cannot tell
// them apart.
type Analyzer interface {
	Propose(v *Vocabulary, records []Record) (candidates []Candidate, uncovered []Record, err error)
}

// SequentialCovering is the baseline analyzer: greedy decision-list
// mining. Each round it picks the (feature, action) rule that correctly
// covers the most remaining records — preferring rules with zero
// counterexamples — appends it, and removes what the rule would handle
// at runtime. Fully deterministic: ties break by vocabulary order.
type SequentialCovering struct {
	// MinCoverage discards rules covering fewer records; 0 means 1.
	MinCoverage int
	// MaxRules bounds the list; 0 means 64.
	MaxRules int
	// MaxEvidence bounds the per-candidate evidence references; 0
	// means 5.
	MaxEvidence int
}

var _ Analyzer = SequentialCovering{}

// Propose implements Analyzer.
func (sc SequentialCovering) Propose(v *Vocabulary, records []Record) ([]Candidate, []Record, error) {
	if len(records) == 0 {
		return nil, nil, ErrEmptyCorpus
	}
	minCov := max(sc.MinCoverage, 1)
	maxRules := sc.MaxRules
	if maxRules == 0 {
		maxRules = 64
	}
	maxEv := sc.MaxEvidence
	if maxEv == 0 {
		maxEv = 5
	}

	actionIdx := map[string]int{}
	for i, a := range v.Actions {
		actionIdx[a.Name] = i
	}
	remaining := append([]Record(nil), records...)
	var out []Candidate
	for len(remaining) > 0 && len(out) < maxRules {
		bestF, bestA, bestCorrect, bestCounter := -1, -1, 0, 0
		for fi := range v.Features {
			// Count per action among remaining records where the
			// feature holds.
			counts := make([]int, len(v.Actions))
			total := 0
			for _, r := range remaining {
				if r.Bits[fi] {
					counts[actionIdx[r.Action]]++
					total++
				}
			}
			for ai, correct := range counts {
				if correct < minCov {
					continue
				}
				counter := total - correct
				better := false
				switch {
				case bestF == -1:
					better = true
				case (bestCounter == 0) != (counter == 0):
					better = counter == 0 // clean rules beat dirty ones
				case correct != bestCorrect:
					better = correct > bestCorrect
				case counter != bestCounter:
					better = counter < bestCounter
				}
				if better {
					bestF, bestA, bestCorrect, bestCounter = fi, ai, correct, counter
				}
			}
		}
		if bestF == -1 {
			break // nothing left worth a rule
		}
		cand := Candidate{
			Condition:       v.Features[bestF].Name,
			Action:          v.Actions[bestA].Name,
			Priority:        len(out),
			Coverage:        bestCorrect,
			Counterexamples: bestCounter,
			Rationale: fmt.Sprintf("when %s held, %s was taken %d/%d times at this list position",
				v.Features[bestF].Name, v.Actions[bestA].Name, bestCorrect, bestCorrect+bestCounter),
		}
		// Split remaining: the rule handles everything its condition
		// matches from here on, so those records leave the pool —
		// matches with a different action are the recorded
		// counterexamples (concept:behavior-evidence).
		var rest []Record
		for _, r := range remaining {
			if !r.Bits[bestF] {
				rest = append(rest, r)
				continue
			}
			if r.Action == cand.Action && len(cand.Evidence) < maxEv {
				cand.Evidence = append(cand.Evidence, Evidence{Episode: r.Episode, Tick: r.Tick})
			}
		}
		remaining = rest
		out = append(out, cand)
	}
	return out, remaining, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
