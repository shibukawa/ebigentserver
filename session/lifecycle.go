package session

import "fmt"

// State is one concept:session-lifecycle state. The transition table below
// is the concept's, verbatim; a terminal transition occurs exactly once.
type State uint8

const (
	// StateCreated: configuration validated; no agents admitted.
	StateCreated State = iota
	// StateAdmitting: pre-start agents may join; simulation not committed.
	StateAdmitting
	// StateRunning: actions and ticks accepted.
	StateRunning
	// StateDraining: new admission and actions rejected; final outputs
	// flush until deadline.
	StateDraining
	// StateEnded: normal terminal state.
	StateEnded
	// StateAborted: unrecoverable-failure terminal state.
	StateAborted
)

// Terminal reports whether the state is one of the two terminal states.
func (s State) Terminal() bool { return s == StateEnded || s == StateAborted }

// CanTransition reports whether the lifecycle permits moving to next.
func (s State) CanTransition(next State) bool {
	switch {
	case s == StateCreated && next == StateAdmitting:
		return true // initialization succeeded
	case s == StateAdmitting && next == StateRunning:
		return true // game start condition succeeded
	case s == StateRunning && next == StateDraining:
		return true // game end, operator stop, or process shutdown
	case s == StateDraining && next == StateEnded:
		return true // final outputs and lifecycle callbacks finished
	case next == StateAborted && !s.Terminal():
		return true // invariant violation or unrecoverable failure
	default:
		return false
	}
}

func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateAdmitting:
		return "admitting"
	case StateRunning:
		return "running"
	case StateDraining:
		return "draining"
	case StateEnded:
		return "ended"
	case StateAborted:
		return "aborted"
	default:
		return fmt.Sprintf("State(%d)", uint8(s))
	}
}
