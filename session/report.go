package session

// Report is data:progress-report: the one outbound stream from a session
// to the control plane, carrying both incremental game facts and the
// terminal result in the same envelope. Phase 1 emits only the
// end-of-session reports; the emission point exists now so control-plane
// integration is not a cross-cutting retrofit (plan.md seam table).
type Report struct {
	// SessionID plus Seq is the idempotency key for redelivery.
	SessionID string
	Seq       uint64
	// Tick is the commit count at emission.
	Tick Tick
	// Subject is the slot involved; 0 means the session itself.
	Subject SlotID
	// Kind is the game- or framework-defined name of the fact.
	// The framework emits "slot_outcome" and "session_ended".
	Kind string
	// Outcome accompanies "slot_outcome" reports.
	Outcome Terminal
	// Terminal is true only on the final report, which closes the
	// session record.
	Terminal bool
}

// ReportSink receives a session's reports in emission order. Delivery
// guarantees (buffering, resend until acknowledged) belong to the control
// plane integration, not to Phase 1.
type ReportSink interface {
	Report(r Report) error
}

// Discard drops every report: the default sink for sessions that have no
// control plane.
type Discard struct{}

// Report does nothing.
func (Discard) Report(Report) error { return nil }
