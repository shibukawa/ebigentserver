package runconf

import (
	"errors"
	"fmt"
	"slices"
)

// Accepted values, mirroring the enum tags on the structs above.
//
// They are duplicated here on purpose. In tinybind-go v0.5.17 the enum
// tag reaches neither the generated code nor the loader — generation
// consults it only when checking a dependon condition's values — so a
// value outside the list loads without complaint and only this check
// catches it. Keep the two in step when either moves.
var (
	topologies    = []string{"standalone", "listen", "dedicated", "p2p"}
	syncModes     = []string{"delay", "rollback", "server_authoritative", "hybrid"}
	baselineModes = []string{"speculative", "confirmed_only", "bounded_speculation"}
	ackModes      = []string{"piggyback_only", "dedicated", "delayed_piggyback"}
	timeModes     = []string{"realtime", "scaled", "step", "unlimited"}
	episodeModes  = []string{"replay_complete", "analysis_sampled"}
	slotKinds     = []string{"human", "script", "behavior_tree", "llm", "replay", "remote"}
)

// oneOf records an error when value is outside allowed.
func oneOf(errs []error, key, value string, allowed []string) []error {
	if slices.Contains(allowed, value) {
		return errs
	}
	return append(errs, fmt.Errorf("%s is %q, not one of %v", key, value, allowed))
}

// Validate checks the run declaration for consistency that no single
// field can express. Values outside an enum tag are already rejected by
// the load; what remains is agreement between fields.
//
// It runs at startup and rejects there, never at first use — the same
// contract budget.Budget and session.TuningProfile hold to.
func (r Run) Validate() error {
	var errs []error

	errs = oneOf(errs, "run.topology", r.Topology, topologies)
	errs = oneOf(errs, "run.sync.mode", r.Sync.Mode, syncModes)
	errs = oneOf(errs, "run.sync.baseline", r.Sync.Baseline, baselineModes)
	errs = oneOf(errs, "run.sync.ack", r.Sync.Ack, ackModes)
	errs = oneOf(errs, "run.time.mode", r.Time.Mode, timeModes)
	if r.Episode.Dir != "" {
		errs = oneOf(errs, "run.episode.mode", r.Episode.Mode, episodeModes)
	}

	switch r.Topology {
	case "listen", "dedicated":
		if r.Listen == "" {
			errs = append(errs, fmt.Errorf("topology %q needs run.listen", r.Topology))
		}
	case "standalone":
		if r.Listen != "" {
			errs = append(errs, errors.New("standalone opens no listener, so run.listen must stay empty"))
		}
	}

	if r.Sync.Baseline == "bounded_speculation" && r.Sync.SpeculationDepth < 1 {
		errs = append(errs, errors.New("baseline bounded_speculation needs run.sync.speculation_depth of at least 1"))
	}
	if r.Sync.Baseline != "bounded_speculation" && r.Sync.SpeculationDepth != 0 {
		errs = append(errs, fmt.Errorf("run.sync.speculation_depth applies only to bounded_speculation, not %q", r.Sync.Baseline))
	}

	if r.Time.Mode == "scaled" && r.Time.ScalePermille <= 0 {
		errs = append(errs, errors.New("time mode scaled needs a positive run.time.scale_permille"))
	}

	seen := map[int]bool{}
	for i, s := range r.Slot {
		if !slices.Contains(slotKinds, s.Kind) {
			errs = append(errs, fmt.Errorf("run.slot[%d].kind %q is not one of %v", i, s.Kind, slotKinds))
		}
		if s.Index < 0 {
			errs = append(errs, fmt.Errorf("run.slot[%d].index must not be negative", i))
		}
		if seen[s.Index] {
			errs = append(errs, fmt.Errorf("run.slot[%d] repeats index %d; one controller fills one slot", i, s.Index))
		}
		seen[s.Index] = true
		switch s.Kind {
		case "behavior_tree", "replay", "script":
			if s.Source == "" {
				errs = append(errs, fmt.Errorf("run.slot[%d] kind %q needs a source", i, s.Kind))
			}
		}
	}

	if r.Episode.Dir != "" {
		if r.Episode.SamplePercent < 1 || r.Episode.SamplePercent > 100 {
			errs = append(errs, fmt.Errorf("run.episode.sample_percent is %d, outside 1..100", r.Episode.SamplePercent))
		}
		if r.Episode.Mode == "replay_complete" && r.Episode.SamplePercent != 100 {
			// Sampling selects sessions, never ticks, so a sampled
			// replay_complete corpus is still lossless per selected
			// session. Saying so out loud is cheaper than the reader
			// assuming the opposite.
			errs = append(errs, errors.New("replay_complete with sample_percent below 100 records complete episodes for only some sessions; set sample_percent to 100 or use analysis_sampled"))
		}
	}

	if r.EvaluationVersion < 1 {
		errs = append(errs, errors.New("run.evaluation_version must be at least 1"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("runconf: invalid run configuration: %w", errors.Join(errs...))
	}
	return nil
}
