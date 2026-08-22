package runconf

import (
	"errors"
	"fmt"
	"slices"
)

// Accepted values, mirroring the enum tags on the structs above.
//
// They are duplicated here on purpose. In tinybind-go the enum tag
// reaches neither the generated code nor the loader — generation consults
// it only when checking a dependon condition's values — so a value
// outside the list loads without complaint and only this check catches
// it. Keep the two in step when either moves.
var (
	topologies    = []string{"standalone", "listen", "dedicated", "p2p"}
	baselineModes = []string{"speculative", "confirmed_only", "bounded_speculation"}
	ackModes      = []string{"piggyback_only", "dedicated", "delayed_piggyback"}
	timeModes     = []string{"realtime", "scaled", "step", "unlimited"}
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
	errs = oneOf(errs, "run.time.mode", r.Time.Mode, timeModes)

	switch r.Topology {
	case "listen", "dedicated":
		if r.Listen == "" {
			errs = append(errs, fmt.Errorf("topology %q needs run.listen", r.Topology))
		}
	case "standalone":
		if r.Listen != "" {
			errs = append(errs, errors.New("standalone opens no listener, so run.listen must stay empty"))
		}
		if r.Server != "" {
			errs = append(errs, errors.New("standalone joins nobody, so run.server must stay empty"))
		}
	}
	// Binding and dialing are the two halves of one question, and a
	// process that does both is either talking to itself or has been
	// handed two answers by mistake.
	if r.Listen != "" && r.Server != "" {
		errs = append(errs, errors.New("run.listen and run.server are the two sides of one link; set the one this process takes"))
	}

	if r.Time.Mode == "scaled" && r.Time.ScalePermille <= 0 {
		errs = append(errs, errors.New("time mode scaled needs a positive run.time.scale_permille"))
	}

	errs = append(errs, r.Tuning.validate()...)

	if len(errs) > 0 {
		return fmt.Errorf("runconf: invalid run configuration: %w", errors.Join(errs...))
	}
	return nil
}

// validate checks data:session-tuning-profile for the agreement between
// fields that no one of them can express. The framework declares no
// defaults for the rates themselves (decision:no-framework-tuning-defaults),
// but it does insist the numbers are consistent with each other.
func (t Tuning) validate() []error {
	var errs []error

	errs = oneOf(errs, "run.tuning.baseline", t.Baseline, baselineModes)
	errs = oneOf(errs, "run.tuning.ack", t.Ack, ackModes)

	switch {
	case t.TickRate < 1:
		errs = append(errs, fmt.Errorf("run.tuning.tick_rate is %d, and a session has to step", t.TickRate))
	case t.SendRate < 1:
		errs = append(errs, fmt.Errorf("run.tuning.send_rate is %d, and a peer has to hear something", t.SendRate))
	case t.SendRate > t.TickRate:
		errs = append(errs, fmt.Errorf("run.tuning.send_rate %d is above tick_rate %d; a session cannot send a state it has not stepped", t.SendRate, t.TickRate))
	case t.TickRate%t.SendRate != 0:
		// A fractional cadence would put a send between two ticks,
		// which is a state nothing committed.
		errs = append(errs, fmt.Errorf("run.tuning.tick_rate %d is not a whole multiple of send_rate %d", t.TickRate, t.SendRate))
	}

	if t.SnapshotEvery < 0 {
		errs = append(errs, fmt.Errorf("run.tuning.snapshot_every is %d, which is not a cadence", t.SnapshotEvery))
	}
	if t.HistoryDepth < 1 {
		errs = append(errs, fmt.Errorf("run.tuning.history_depth is %d, and a delta needs a baseline to be computed against", t.HistoryDepth))
	}

	switch {
	case t.Baseline == "bounded_speculation" && t.SpeculationDepth < 1:
		errs = append(errs, errors.New("baseline bounded_speculation needs run.tuning.speculation_depth of at least 1"))
	case t.Baseline != "bounded_speculation" && t.SpeculationDepth != 0:
		errs = append(errs, fmt.Errorf("run.tuning.speculation_depth applies only to bounded_speculation, not %q", t.Baseline))
	case t.Baseline == "bounded_speculation" && t.HistoryDepth > 0 && t.SpeculationDepth >= t.HistoryDepth:
		// Speculating past what is retained leaves the baseline
		// unavailable when the delta is finally computed.
		errs = append(errs, fmt.Errorf("run.tuning.speculation_depth %d must stay below history_depth %d", t.SpeculationDepth, t.HistoryDepth))
	}

	return errs
}
