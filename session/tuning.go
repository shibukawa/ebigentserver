package session

import (
	"errors"
	"fmt"
)

// TuningProfile is data:session-tuning-profile: the declared parameter set
// fixing the timing and bandwidth behavior of one game. The framework
// ships no defaults (decision:no-framework-tuning-defaults) — a fighting
// game, a shooter, and a strategy game disagree on every one of these
// values, so every realtime session must declare its own profile and the
// declaration is validated for internal consistency before play.
//
// Phase 3a uses the timing and retention fields; bandwidth, ack, and lag
// compensation fields gain their consumers in later phases but are
// declared here so profiles do not churn shape.
type TuningProfile struct {
	// TickRate is simulation steps per second. Required.
	TickRate int32
	// SendRate is downstream state updates per second, at most TickRate.
	// Required. TickRate must be a multiple of SendRate so the send
	// cadence is a whole number of ticks.
	SendRate int32
	// SnapshotEvery sends a full data:snapshot every N-th update (the
	// snapshot_interval cadence); between them deltas flow. 0 sends
	// snapshots only on join and resync.
	SnapshotEvery int32
	// HistoryDepth is how many committed world versions the sender
	// retains per receiver — the bound on both delta baselines
	// (rule:delta-baseline-must-be-retained) and, later, lag
	// compensation. Required, at least 1.
	HistoryDepth int32
	// MaxSnapshotSize bounds one encoded snapshot in bytes; 0 leaves it
	// unchecked until api:message-framing consumes it in Phase 3b.
	MaxSnapshotSize int32
	// BandwidthBudget is target bytes per second per receiver; 0 leaves
	// it unchecked until Phase 3b measures real links.
	BandwidthBudget int32
	// SilenceDeadline is expressed in missed ticks, not seconds; 0
	// until Phase 3b's silence detection consumes it.
	SilenceDeadline int32
}

// Validate checks the declaration for presence and internal consistency.
func (p TuningProfile) Validate() error {
	var errs []error
	if p.TickRate <= 0 {
		errs = append(errs, errors.New("TickRate must be declared and positive"))
	}
	if p.SendRate <= 0 {
		errs = append(errs, errors.New("SendRate must be declared and positive"))
	}
	if p.HistoryDepth < 1 {
		errs = append(errs, errors.New("HistoryDepth must be declared and at least 1"))
	}
	if p.TickRate > 0 && p.SendRate > 0 {
		if p.SendRate > p.TickRate {
			errs = append(errs, fmt.Errorf("SendRate %d exceeds TickRate %d", p.SendRate, p.TickRate))
		} else if p.TickRate%p.SendRate != 0 {
			errs = append(errs, fmt.Errorf("TickRate %d is not a multiple of SendRate %d", p.TickRate, p.SendRate))
		}
	}
	if p.SnapshotEvery < 0 || p.MaxSnapshotSize < 0 || p.BandwidthBudget < 0 || p.SilenceDeadline < 0 {
		errs = append(errs, errors.New("no tuning value may be negative"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("session: invalid tuning profile: %w", errors.Join(errs...))
	}
	return nil
}

// SendEvery returns the send cadence in ticks.
func (p TuningProfile) SendEvery() Tick { return Tick(p.TickRate / p.SendRate) }
