// Package observe implements api:runtime-observability in its smallest
// honest form: bounded-label counters and structured events with the
// required context fields. Every shed, reject, disconnect, and abort in
// the hosting layers emits evidence here (policy:overload-handling's
// last requirement); credentials and full sights never do.
package observe

import (
	"sync"
	"sync/atomic"
)

// Metrics is the bounded-cardinality counter set. All methods are safe
// for concurrent use; a nil *Metrics is a no-op sink so instrumentation
// never needs a nil check.
type Metrics struct {
	ActiveConnections atomic.Int64
	TicksCommitted    atomic.Int64
	InputsAccepted    atomic.Int64
	InputsRejected    atomic.Int64
	AdmissionRejected atomic.Int64
	Disconnects       atomic.Int64
	OverloadSheds     atomic.Int64
	ResyncRequests    atomic.Int64
}

// Snapshot is a point-in-time copy for reporting.
type Snapshot struct {
	ActiveConnections int64
	TicksCommitted    int64
	InputsAccepted    int64
	InputsRejected    int64
	AdmissionRejected int64
	Disconnects       int64
	OverloadSheds     int64
	ResyncRequests    int64
}

// Read copies the counters.
func (m *Metrics) Read() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	return Snapshot{
		ActiveConnections: m.ActiveConnections.Load(),
		TicksCommitted:    m.TicksCommitted.Load(),
		InputsAccepted:    m.InputsAccepted.Load(),
		InputsRejected:    m.InputsRejected.Load(),
		AdmissionRejected: m.AdmissionRejected.Load(),
		Disconnects:       m.Disconnects.Load(),
		OverloadSheds:     m.OverloadSheds.Load(),
		ResyncRequests:    m.ResyncRequests.Load(),
	}
}

// Event is one structured operational event, carrying the required
// context of api:runtime-observability. Reason is a stable code, not
// prose; identifiers belong here (logs), not in metric labels.
type Event struct {
	SessionID string
	// ConnID identifies the connection where one is involved; 0 slot
	// events are session-scoped.
	ConnID string
	Slot   uint16
	Tick   uint64
	// Kind: admission_reject, abuse_reject, input_reject, disconnect,
	// overload_shed, departure, takeover, session_end, session_abort,
	// checkpoint_mismatch.
	Kind string
	// Reason is the stable reason code.
	Reason string
}

// Log collects events. A nil *Log drops them; a bounded ring keeps the
// newest events for inspection.
type Log struct {
	mu     sync.Mutex
	events []Event
	cap    int
}

// NewLog builds a ring keeping the newest capacity events.
func NewLog(capacity int) *Log { return &Log{cap: capacity} }

// Emit appends one event.
func (l *Log) Emit(ev Event) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, ev)
	if l.cap > 0 && len(l.events) > l.cap {
		l.events = l.events[len(l.events)-l.cap:]
	}
}

// Events copies the retained events.
func (l *Log) Events() []Event {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Event, len(l.events))
	copy(out, l.events)
	return out
}

// CountKind reports how many retained events carry the kind.
func (l *Log) CountKind(kind string) int {
	n := 0
	for _, ev := range l.Events() {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}
