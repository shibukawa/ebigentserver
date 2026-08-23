package session

// This file is the session side of episode recording: the hook points the
// session drives and the checkpoint type proving deterministic commits
// (data:state-checkpoint). The JSONL materialization of these hooks lives
// in the episode package; the session itself never formats a log line
// (rule:session-independent-of-transport-and-agent-kind applies to sinks
// too — the session cannot know whether it is being recorded to disk,
// sampled, or ignored).

// EpisodeStart is delivered once, before the first sight.
type EpisodeStart struct {
	// SessionID names the episode's session.
	SessionID string
	// Seed is the session's shared RNG seed (rule:shared-rng-seed),
	// stored so a replay reproduces exactly.
	Seed uint64
	// Slots is the slot set in commit order.
	Slots []SlotID
}

// SlotOutcome is one slot's final signal, emitted at episode end.
type SlotOutcome struct {
	Slot   SlotID
	Signal EvaluationSignal
}

// Checkpoint is data:state-checkpoint: a deterministic digest proving that
// independent executions reached the same committed state. Unequal hashes
// at equal tick is a determinism error, never a tolerance discussion.
type Checkpoint struct {
	// Tick is the commit the digest covers.
	Tick Tick
	// WorldHash digests the canonical encoding of the committed world
	// state. The game's RNG position is part of its state, so it is
	// covered here (rng_position of data:state-checkpoint).
	WorldHash uint64
	// ActionHash is a running digest over every accepted action so far,
	// in commit order.
	ActionHash uint64
}

// Recorder receives everything a replay needs (concept:episode-recording-
// mode, replay_complete: every delivered sight, accepted action,
// lifecycle transition, rng seed, and periodic checkpoints). A nil
// Config.Recorder records nothing at zero cost.
//
// Calls arrive in deterministic commit order on the session's goroutine;
// implementations must not block for long.
type Recorder[W, A, S any] interface {
	// EpisodeStarted opens the episode.
	EpisodeStarted(start EpisodeStart)
	// Observed is one delivered sight for a slot that is not
	// deciding this tick (data:decision-record with action none).
	Observed(tick Tick, slot SlotID, obs S, sig EvaluationSignal)
	// Decided is one accepted action together with the sight as
	// delivered at the decision point — the sight is the record
	// (data:decision-record). latencyMicros is wall-clock decision
	// latency: measurement metadata, never simulation input.
	Decided(tick Tick, slot SlotID, obs S, action A, sig EvaluationSignal, latencyMicros int64)
	// Rejected is one action the validator refused (worth keeping: a
	// cluster of rejections is a cheat or a client bug).
	Rejected(tick Tick, slot SlotID, reason string)
	// Lifecycle is one state transition of concept:session-lifecycle.
	Lifecycle(tick Tick, from, to State)
	// WorldCommitted delivers the committed world state after a tick,
	// for the optional ground-truth stream.
	WorldCommitted(tick Tick, world *W)
	// Checkpointed delivers a data:state-checkpoint.
	Checkpointed(cp Checkpoint)
	// Ended closes the episode with every slot's outcome.
	Ended(tick Tick, outcomes []SlotOutcome)
}
