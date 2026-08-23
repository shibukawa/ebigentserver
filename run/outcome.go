package run

import "github.com/shibukawa/ebigentserver/session"

// Watcher wraps a session.Recorder to keep each slot's final
// data:evaluation-signal, so a match can report how it went without the
// caller re-reading the episode it just wrote. Every other hook passes
// straight through, including to a nil inner recorder, which is what lets
// the wrapper attach one whether or not anything is being recorded.
type Watcher[W, A, S any] struct {
	inner    session.Recorder[W, A, S]
	outcomes []session.SlotOutcome
}

var _ session.Recorder[struct{}, struct{}, struct{}] = (*Watcher[struct{}, struct{}, struct{}])(nil)

// Watch wraps a recorder, which may be nil.
func Watch[W, A, S any](inner session.Recorder[W, A, S]) *Watcher[W, A, S] {
	return &Watcher[W, A, S]{inner: inner}
}

// Outcomes reports the final signals, valid once the match has ended.
func (s *Watcher[W, A, S]) Outcomes() []session.SlotOutcome { return s.outcomes }

func (s *Watcher[W, A, S]) EpisodeStarted(start session.EpisodeStart) {
	if s.inner != nil {
		s.inner.EpisodeStarted(start)
	}
}

func (s *Watcher[W, A, S]) Observed(tick session.Tick, slot session.SlotID, obs S, sig session.EvaluationSignal) {
	if s.inner != nil {
		s.inner.Observed(tick, slot, obs, sig)
	}
}

func (s *Watcher[W, A, S]) Decided(tick session.Tick, slot session.SlotID, obs S, action A, sig session.EvaluationSignal, latencyMicros int64) {
	if s.inner != nil {
		s.inner.Decided(tick, slot, obs, action, sig, latencyMicros)
	}
}

func (s *Watcher[W, A, S]) Rejected(tick session.Tick, slot session.SlotID, reason string) {
	if s.inner != nil {
		s.inner.Rejected(tick, slot, reason)
	}
}

func (s *Watcher[W, A, S]) Lifecycle(tick session.Tick, from, to session.State) {
	if s.inner != nil {
		s.inner.Lifecycle(tick, from, to)
	}
}

func (s *Watcher[W, A, S]) WorldCommitted(tick session.Tick, world *W) {
	if s.inner != nil {
		s.inner.WorldCommitted(tick, world)
	}
}

func (s *Watcher[W, A, S]) Checkpointed(cp session.Checkpoint) {
	if s.inner != nil {
		s.inner.Checkpointed(cp)
	}
}

func (s *Watcher[W, A, S]) Ended(tick session.Tick, outcomes []session.SlotOutcome) {
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
