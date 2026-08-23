package session

import "context"

// Detached fills a slot whose real controller lives on the far side of a
// link — the session-side face of what Phase 3b formalizes as
// actor:remote-agent. The session's direct callbacks are no-ops: the
// controller receives its sights through the downstream state
// stream and submits actions through the slot's Inbox, so delivering
// them here too would duplicate (and race with) the real path.
type Detached[S, A any] struct{}

var _ Agent[struct{}, struct{}] = Detached[struct{}, struct{}]{}

// Joined does nothing.
func (Detached[S, A]) Joined(SlotID) {}

// Observe does nothing; sights travel the state stream.
func (Detached[S, A]) Observe(S) {}

// Decide reports no action; realtime intake reads the Inbox instead.
func (Detached[S, A]) Decide(context.Context) (a A, ok bool) { return a, false }

// Ended does nothing; the link closing tells the controller.
func (Detached[S, A]) Ended(Result) {}
