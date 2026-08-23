// Package episode materializes a session's record hooks as data:episode-log:
// JSONL streams that analysis tools read directly — one JSON object per
// line, UTF-8, append-only, crash-safe at line granularity.
//
// The four streams keep separate stable column sets (columnar readers
// handle that far better than one mixed record type):
//
//   - decisions: data:decision-record rows, the primary analysis table
//   - events: lifecycle transitions, validator rejections, checkpoints
//   - outcomes: one row per slot at episode end (metric:episode-outcome)
//   - world: optional full world-state ground truth for debugging
//
// Every stream begins with the same header row carrying the schema
// version, protocol version, RNG seed (rule:shared-rng-seed), evaluation
// version, and the recording mode (concept:episode-recording-mode).
package episode

import (
	"encoding/json"
	"fmt"
	"io"
)

// SchemaVersion is the version of the log row shapes defined here.
const SchemaVersion = 1

// Mode is concept:episode-recording-mode: the contract separating lossless
// replay evidence from sampled analysis data.
type Mode string

const (
	// ReplayComplete requires every delivered sight, accepted
	// action, lifecycle transition, the RNG seed, and periodic
	// checkpoints: a replay agent can verify the episode without
	// unrecorded game decisions.
	ReplayComplete Mode = "replay_complete"
	// AnalysisSampled may skip the world stream and checkpoints and
	// sample decision points. Suitable for aggregate analysis only;
	// never labeled replayable.
	AnalysisSampled Mode = "analysis_sampled"
)

// Header is the first row of every stream.
type Header struct {
	Stream            string `json:"stream"`
	SchemaVersion     int    `json:"schema_version"`
	ProtocolVersion   string `json:"protocol_version,omitempty"`
	EpisodeID         string `json:"episode_id"`
	Mode              Mode   `json:"mode"`
	Seed              uint64 `json:"seed"`
	EvaluationVersion int    `json:"evaluation_version"`
}

// Evaluation mirrors session.EvaluationSignal into stable JSON columns.
// Fixed-point fields serialize as their raw int64 bits.
type Evaluation struct {
	Score       int64  `json:"score"`
	Progress    int64  `json:"progress"`
	Evaluation  int64  `json:"evaluation"`
	RewardDelta int64  `json:"reward_delta"`
	Terminal    string `json:"terminal,omitempty"`
}

// Decision is one row of the decisions stream (data:decision-record): what
// an agent could see, what it did, and how the position was judged. A row
// with a null action is a delivered sight at a step where the slot
// did not decide.
type Decision struct {
	Tick uint64 `json:"tick"`
	Slot uint16 `json:"slot"`
	// AgentKind separates human and bot rows in analysis; filled from
	// the writer's slot-kind table, empty when unknown.
	AgentKind string `json:"agent_kind,omitempty"`
	// Sight is the concept:sight as delivered — never the world state.
	Sight json.RawMessage `json:"sight"`
	// Action is the action taken, or null.
	Action json.RawMessage `json:"action,omitempty"`
	// Evaluation is the data:evaluation-signal at this decision point.
	Evaluation Evaluation `json:"evaluation"`
	// LatencyMicros is wall-clock decision latency; sight-only
	// rows carry 0.
	LatencyMicros int64 `json:"latency_micros,omitempty"`
}

// Event is one row of the events stream.
type Event struct {
	Tick uint64 `json:"tick"`
	// Kind is "lifecycle", "rejected", or "checkpoint".
	Kind string `json:"kind"`
	// From and To carry lifecycle transitions.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Slot and Reason carry rejections.
	Slot   uint16 `json:"slot,omitempty"`
	Reason string `json:"reason,omitempty"`
	// WorldHash and ActionHash carry checkpoints
	// (data:state-checkpoint), hex so uint64 survives JSON readers.
	WorldHash  string `json:"world_hash,omitempty"`
	ActionHash string `json:"action_hash,omitempty"`
}

// Outcome is one row of the outcomes stream (metric:episode-outcome).
type Outcome struct {
	Slot uint16 `json:"slot"`
	// Result is the terminal field of the slot's final signal.
	Result string `json:"result"`
	// Reward accumulates the recorded reward_delta values (raw fixed
	// point bits).
	Reward int64 `json:"reward"`
	// DurationTicks is the episode length.
	DurationTicks uint64 `json:"duration_ticks"`
}

// World is one row of the world stream: full ground truth for debugging,
// never joined into behavior analysis by default.
type World struct {
	Tick  uint64          `json:"tick"`
	State json.RawMessage `json:"state"`
}

func writeLine(w io.Writer, v any) error {
	if w == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("episode: marshal %T: %w", v, err)
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
