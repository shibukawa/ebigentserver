package behavior

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// EvalReport is what a decision list did against records it was not
// mined from: the validate half of requirement:corpus-curation.
type EvalReport struct {
	// Covered records got the recorded action from the first matching
	// chip; Misplayed records matched a chip that answered differently;
	// Silent records matched no approved chip at all.
	Covered   []Record
	Misplayed []Record
	Silent    []Record
}

// Evaluate replays the approved chips over records, in the exact order
// GenerateAgent compiles them. Run it against a holdout split and the
// three buckets say what the training corpus could not: Silent is the
// gap list — the situations to collect in the next play or simulate
// round — and Misplayed is where the mined order disagrees with play the
// miner never saw.
func Evaluate(v *Vocabulary, lib *Library, records []Record) (EvalReport, error) {
	features := map[string]int{}
	for i, f := range v.Features {
		features[f.Name] = i
	}
	chips := lib.Approved()
	sort.SliceStable(chips, func(i, j int) bool { return chips[i].Priority < chips[j].Priority })
	idx := make([]int, len(chips))
	for i, chip := range chips {
		fi, ok := features[chip.Condition]
		if !ok {
			return EvalReport{}, fmt.Errorf("behavior: chip %q names unknown predicate %q (stale vocabulary fails the build, as it should)", chip.Key(), chip.Condition)
		}
		idx[i] = fi
	}

	var rep EvalReport
	for _, r := range records {
		answered := false
		for i, chip := range chips {
			if !r.Bits[idx[i]] {
				continue
			}
			if chip.Action == r.Action {
				rep.Covered = append(rep.Covered, r)
			} else {
				rep.Misplayed = append(rep.Misplayed, r)
			}
			answered = true
			break
		}
		if !answered {
			rep.Silent = append(rep.Silent, r)
		}
	}
	return rep, nil
}

// gapRow is one silent moment in gaps.jsonl.
type gapRow struct {
	Episode string          `json:"episode"`
	Tick    uint64          `json:"tick"`
	Slot    uint16          `json:"slot"`
	Sight   json.RawMessage `json:"sight"`
	// Action is the recorded action's vocabulary name: what was played
	// in the situation the policy has no answer for.
	Action string `json:"action"`
}

// WriteGaps writes the silent records as JSONL, one replayable moment
// per line. This is the gaps.jsonl of data:curated-corpus: the shopping
// list the next recording round works from.
func (r EvalReport) WriteGaps(w io.Writer) error {
	enc := json.NewEncoder(w)
	for _, rec := range r.Silent {
		row := gapRow{Episode: rec.Episode, Tick: rec.Tick, Slot: rec.Slot, Sight: rec.Obs, Action: rec.Action}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
