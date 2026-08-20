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
	// BaselineMode selects which retained version deltas are computed
	// against (concept:delta-baseline-policy).
	BaselineMode BaselineMode
	// SpeculationDepth bounds how far past the confirmed baseline
	// BaselineBounded may speculate; required for that mode.
	SpeculationDepth int32
	// AckMode selects how ack records reach the peer
	// (concept:ack-transmission-policy): 0 piggyback_only, 1 dedicated,
	// 2 delayed_piggyback. Consumed by the transport frontend.
	AckMode uint8
}

// BaselineMode is concept:delta-baseline-policy.
type BaselineMode uint8

const (
	// BaselineSpeculative diffs against the last sent version: minimum
	// bandwidth, but one lost message invalidates the chain until
	// resync. The right declaration for loss-free local links.
	BaselineSpeculative BaselineMode = iota
	// BaselineConfirmedOnly diffs against the newest version the peer
	// acked: every delta decodes on arrival, at the cost of deltas that
	// grow with RTT and loss.
	BaselineConfirmedOnly
	// BaselineBounded speculates up to SpeculationDepth versions past
	// the confirmed baseline, then falls back to it.
	BaselineBounded
)

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
	if p.SnapshotEvery < 0 || p.MaxSnapshotSize < 0 || p.BandwidthBudget < 0 || p.SilenceDeadline < 0 || p.SpeculationDepth < 0 {
		errs = append(errs, errors.New("no tuning value may be negative"))
	}
	if p.BaselineMode == BaselineBounded && p.SpeculationDepth < 1 {
		errs = append(errs, errors.New("BaselineBounded requires SpeculationDepth"))
	}
	if p.BaselineMode == BaselineBounded && p.HistoryDepth > 0 && p.SpeculationDepth >= p.HistoryDepth {
		errs = append(errs, errors.New("SpeculationDepth must stay below HistoryDepth so the confirmed fallback is still retained"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("session: invalid tuning profile: %w", errors.Join(errs...))
	}
	return nil
}

// SendEvery returns the send cadence in ticks.
func (p TuningProfile) SendEvery() Tick { return Tick(p.TickRate / p.SendRate) }
