// Package budget declares data:runtime-resource-budget: the hard bounds
// that keep one game process, session, connection, and decoder within
// finite resources. Bounds are declared per game — the framework ships
// none — and validated at startup: a missing, zero, or contradictory
// bound is a configuration error, never a silent unlimited.
//
// Tuning (data:session-tuning-profile) chooses behavior within these
// ceilings; tuning can never raise them. When a bound is hit,
// policy:overload-handling's degradation order applies.
package budget

import (
	"errors"
	"fmt"
)

// Budget is the declared bound set. Every field a deployment relies on
// must be positive; Validate rejects zero values for the fields the
// current phase enforces.
type Budget struct {
	// Process bounds.
	MaxSessions    int32
	MaxConnections int32

	// Session bounds.
	MaxAgents         int32
	MaxPendingActions int32

	// Connection bounds.
	AdmissionPerSecond  int32
	InputsPerTick       int32
	InputBytesPerSecond int32
	SendQueueBytes      int32

	// Decoder bounds (consumed by transport framing and CBOR options).
	MaxMessageSize       int32
	MaxPendingReassembly int32

	// Shutdown bounds.
	DrainDeadlineMillis int32
}

// Validate rejects missing or contradictory bounds
// (data:runtime-resource-budget: reject at startup, not at first use).
func (b Budget) Validate() error {
	var errs []error
	require := func(name string, v int32) {
		if v <= 0 {
			errs = append(errs, fmt.Errorf("%s must be declared and positive", name))
		}
	}
	require("MaxSessions", b.MaxSessions)
	require("MaxConnections", b.MaxConnections)
	require("MaxAgents", b.MaxAgents)
	require("MaxPendingActions", b.MaxPendingActions)
	require("AdmissionPerSecond", b.AdmissionPerSecond)
	require("InputsPerTick", b.InputsPerTick)
	require("InputBytesPerSecond", b.InputBytesPerSecond)
	require("SendQueueBytes", b.SendQueueBytes)
	require("MaxMessageSize", b.MaxMessageSize)
	require("MaxPendingReassembly", b.MaxPendingReassembly)
	require("DrainDeadlineMillis", b.DrainDeadlineMillis)
	if b.MaxAgents > b.MaxConnections {
		errs = append(errs, errors.New("MaxAgents exceeds MaxConnections"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("budget: invalid declaration: %w", errors.Join(errs...))
	}
	return nil
}
