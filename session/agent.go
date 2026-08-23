package session

import "context"

// Agent is api:agent-interface: the contract every controller implements so
// the session can host any agent kind. A human client, a scripted bot, a
// behavior tree, an LLM, and a replay stream all sit behind this same
// interface — game rules can therefore never ask what occupies a slot
// (decision:no-ai-game-mode).
//
// An agent reads only its observation O, never the world state.
//
// Call order per session (concept:session-lifecycle):
// Joined once, before the first Observe; then Observe once per step
// followed by Decide when the slot is acting; then a final Observe of the
// terminal position and Ended exactly once.
type Agent[O, A any] interface {
	// Joined tells the agent which slot it controls. It completes
	// before the first Observe.
	Joined(slot SlotID)
	// Observe delivers the slot's projection of the current step
	// (concept:observation). It never blocks the session for long: in
	// step pacing the session waits in Decide, not here.
	Observe(obs O)
	// Decide returns the agent's next action, or ok=false for none.
	// In step pacing (decision:dual-mode-agent-pacing) the session
	// blocks until Decide returns, and ok=false means the controller
	// cannot continue. ctx is cancelled when the session leaves the
	// running state; a blocked Decide must then return.
	Decide(ctx context.Context) (action A, ok bool)
	// Ended is the session-end callback, delivered after the final
	// Observe. It fires for both normal end and abort.
	Ended(result Result)
}

// Result is what an agent learns when its session reaches a terminal
// lifecycle state.
type Result struct {
	// State is StateEnded or StateAborted.
	State State
	// Signal is the slot's final data:evaluation-signal; its Terminal
	// field carries win, lose, draw, or abandoned.
	Signal EvaluationSignal
}
