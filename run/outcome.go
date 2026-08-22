package run

import "github.com/shibukawa/ebigentserver/session"

// Watcher wraps a session.Recorder to keep each slot's final
// data:evaluation-signal, so a match can report how it went without the
// caller re-reading the episode it just wrote. Every other hook passes
// straight through, including to a nil inner recorder, which is what lets
// the wrapper attach one whether or not anything is being recorded.
type Watcher[S, A, O any] struct {
	inner    session.Recorder[S, A, O]
	outcomes []session.SlotOutcome
}

var _ session.Recorder[struct{}, struct{}, struct{}] = (*Watcher[struct{}, struct{}, struct{}])(nil)

// Watch wraps a recorder, which may be nil.
func Watch[S, A, O any](inner session.Recorder[S, A, O]) *Watcher[S, A, O] {
	return &Watcher[S, A, O]{inner: inner}
}

// Outcomes reports the final signals, valid once the match has ended.
func (s *Watcher[S, A, O]) Outcomes() []session.SlotOutcome { return s.outcomes }

func (s *Watcher[S, A, O]) EpisodeStarted(start session.EpisodeStart) {
	if s.inner != nil {
		s.inner.EpisodeStarted(start)
	}
}

func (s *Watcher[S, A, O]) Observed(tick session.Tick, slot session.SlotID, obs O, sig session.EvaluationSignal) {
	if s.inner != nil {
		s.inner.Observed(tick, slot, obs, sig)
	}
}

func (s *Watcher[S, A, O]) Decided(tick session.Tick, slot session.SlotID, obs O, action A, sig session.EvaluationSignal, latencyMicros int64) {
	if s.inner != nil {
		s.inner.Decided(tick, slot, obs, action, sig, latencyMicros)
	}
}

func (s *Watcher[S, A, O]) Rejected(tick session.Tick, slot session.SlotID, reason string) {
	if s.inner != nil {
		s.inner.Rejected(tick, slot, reason)
	}
}

func (s *Watcher[S, A, O]) Lifecycle(tick session.Tick, from, to session.State) {
	if s.inner != nil {
		s.inner.Lifecycle(tick, from, to)
	}
}

func (s *Watcher[S, A, O]) WorldCommitted(tick session.Tick, world *S) {
	if s.inner != nil {
		s.inner.WorldCommitted(tick, world)
	}
}

func (s *Watcher[S, A, O]) Checkpointed(cp session.Checkpoint) {
	if s.inner != nil {
		s.inner.Checkpointed(cp)
	}
}

func (s *Watcher[S, A, O]) Ended(tick session.Tick, outcomes []session.SlotOutcome) {
	s.outcomes = append(s.outcomes[:0], outcomes...)
	if s.inner != nil {
		s.inner.Ended(tick, outcomes)
	}
}

// Outcome finds one slot's final signal in a result.
func (r MatchResult) Outcome(slot session.SlotID) (session.EvaluationSignal, bool) {
	for _, o := range r.Outcomes {
		if o.Slot == slot {
			return o.Signal, true
		}
	}
	return session.EvaluationSignal{}, false
}
