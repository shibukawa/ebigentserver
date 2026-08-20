package behavior

// Merge folds a fresh proposal round into the library as a diff
// (rule:regeneration-preserves-approved-nodes): approved and rejected
// chips are never silently overwritten — re-analysis produces additions
// and reports, and a contradiction with an approved chip is surfaced for
// an explicit decision, never resolved automatically.

// DiffClass classifies one merge outcome.
type DiffClass string

const (
	// DiffNew is a candidate not previously seen; it enters the
	// library unapproved.
	DiffNew DiffClass = "new"
	// DiffUnchanged matches an existing chip with identical metrics.
	DiffUnchanged DiffClass = "unchanged"
	// DiffMetrics matches an existing chip whose fresh evidence counts
	// moved; the stored chip is untouched, the fresh numbers ride in
	// the diff entry (an approved chip now weakly supported is the
	// "game changed" signal).
	DiffMetrics DiffClass = "changed_metrics"
	// DiffRejectedAgain matches a chip a developer rejected; the old
	// reason is replayed.
	DiffRejectedAgain DiffClass = "matches_rejected"
	// DiffConflict proposes a different action for a condition an
	// approved chip already claims — needs an explicit decision.
	DiffConflict DiffClass = "conflicts_with_approved"
)

// DiffEntry is one line of the regeneration diff.
type DiffEntry struct {
	Class DiffClass
	// Candidate is the fresh proposal.
	Candidate Candidate
	// Existing is the library chip involved, when one is.
	Existing *Chip
}

// Merge applies one proposal round. Only DiffNew mutates the library
// (appending an unapproved chip); everything else is reporting.
func Merge(lib *Library, cands []Candidate) []DiffEntry {
	var diff []DiffEntry
	for _, cand := range cands {
		key := cand.Condition + "→" + cand.Action
		if existing := lib.find(key); existing != nil {
			switch {
			case existing.Rejected:
				diff = append(diff, DiffEntry{Class: DiffRejectedAgain, Candidate: cand, Existing: existing})
			case existing.Coverage == cand.Coverage && existing.Counterexamples == cand.Counterexamples:
				diff = append(diff, DiffEntry{Class: DiffUnchanged, Candidate: cand, Existing: existing})
			default:
				diff = append(diff, DiffEntry{Class: DiffMetrics, Candidate: cand, Existing: existing})
			}
			continue
		}
		if conflict := approvedConditionOwner(lib, cand.Condition); conflict != nil {
			diff = append(diff, DiffEntry{Class: DiffConflict, Candidate: cand, Existing: conflict})
			continue
		}
		lib.Chips = append(lib.Chips, Chip{
			Condition: cand.Condition, Action: cand.Action, Priority: cand.Priority,
			Coverage: cand.Coverage, Counterexamples: cand.Counterexamples,
			Evidence: cand.Evidence, Rationale: cand.Rationale,
		})
		diff = append(diff, DiffEntry{Class: DiffNew, Candidate: cand, Existing: &lib.Chips[len(lib.Chips)-1]})
	}
	return diff
}

func approvedConditionOwner(lib *Library, condition string) *Chip {
	for i := range lib.Chips {
		c := &lib.Chips[i]
		if c.Approved && !c.Rejected && c.Condition == condition {
			return c
		}
	}
	return nil
}
