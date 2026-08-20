package session

// ActionValidator is the legality class of api:action-validator: the
// framework provides the hook position (after the sender is accepted,
// before the simulation applies the action) and the game provides the
// check, since only the game knows what a legal move is.
//
// Legal must be deterministic — under rollback it runs on every simulating
// peer and a disagreement is a desync. The plausibility class (heuristic,
// authoritative-side-only) arrives in Phase 4.
type ActionValidator[S, A any] interface {
	// Legal returns nil when the action is possible under the rules in
	// this state, or an error describing the violation. It must not
	// mutate the state.
	Legal(state *S, slot SlotID, action A) error
}

// AllowAll admits every action: the Phase 1 default keeping the hook
// position occupied (plan.md seam table) until a game supplies checks.
type AllowAll[S, A any] struct{}

// Legal always returns nil.
func (AllowAll[S, A]) Legal(*S, SlotID, A) error { return nil }

// PlausibilityValidator is the second validator class of
// api:action-validator: could an honest client have produced this? It
// runs only on the authoritative side, outside the simulation, so it is
// free to use heuristics — rejecting here never touches simulation state
// and can never desync. A position jump beyond maximum speed or inputs
// stamped for far-future ticks belong here, not in Legal.
type PlausibilityValidator[A any] interface {
	// Plausible returns nil when an honest client could have sent the
	// action at the given tick.
	Plausible(tick Tick, slot SlotID, action A) error
}
