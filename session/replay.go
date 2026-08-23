package session

import "context"

// ReplayAgent is actor:replay-agent: an agent that emits recorded actions
// from a stored episode instead of deciding. Because it sits behind the
// same Agent interface as every other controller, replaying a match is
// just a session whose slots happen to be occupied by replays — no
// special mode exists (decision:no-ai-game-mode covers replays too).
//
// Reproducing identical state from actions alone requires
// term:determinism; the checkpoint stream is what proves it held.
type ReplayAgent[S, A any] struct {
	// Actions is the slot's recorded action sequence, in commit order.
	Actions []A
	next    int
}

var _ Agent[struct{}, struct{}] = (*ReplayAgent[struct{}, struct{}])(nil)

// Joined does nothing.
func (*ReplayAgent[S, A]) Joined(SlotID) {}

// Observe discards the sight: the replayed decisions already
// happened.
func (*ReplayAgent[S, A]) Observe(S) {}

// Decide emits the next recorded action; exhaustion returns no action,
// which drains the session (a truncated recording ends the replay rather
// than inventing moves).
func (r *ReplayAgent[S, A]) Decide(context.Context) (A, bool) {
	if r.next >= len(r.Actions) {
		var zero A
		return zero, false
	}
	a := r.Actions[r.next]
	r.next++
	return a, true
}

// Ended does nothing.
func (*ReplayAgent[S, A]) Ended(Result) {}
