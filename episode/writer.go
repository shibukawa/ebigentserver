package episode

import (
	"encoding/json"
	"fmt"

	"github.com/shibukawa/ebigentserver/session"
)

// Streams holds one destination per JSONL stream. Nil writers drop their
// stream (World is routinely nil outside debugging).
type Streams struct {
	Decisions, Events, Outcomes, World interface{ Write([]byte) (int, error) }
}

// Meta is the header content shared by every stream.
type Meta struct {
	// EpisodeID names the episode; defaults to the session ID when
	// empty.
	EpisodeID string
	// ProtocolVersion is the wire schema identity (data:protocol-version).
	ProtocolVersion string
	// EvaluationVersion versions the game's evaluation function, since
	// changing it invalidates comparisons across a corpus.
	EvaluationVersion int
	// AgentKinds labels slots for the agent_kind column ("human",
	// "bot", "replay", ...). Optional.
	AgentKinds map[session.SlotID]string
}

// Writer materializes session record hooks into JSONL streams. It
// implements session.Recorder.
type Writer[W, A, S any] struct {
	streams Streams
	mode    Mode
	meta    Meta
	err     error // first write error; recording degrades, never aborts play
	reward  map[session.SlotID]int64
}

// NewWriter builds a writer for one episode in the given mode.
// AnalysisSampled drops the world stream and checkpoints regardless of the
// writers supplied, so a sampled log can never masquerade as replayable.
func NewWriter[W, A, S any](streams Streams, mode Mode, meta Meta) *Writer[W, A, S] {
	if mode != ReplayComplete && mode != AnalysisSampled {
		mode = AnalysisSampled
	}
	if mode == AnalysisSampled {
		streams.World = nil
	}
	return &Writer[W, A, S]{
		streams: streams,
		mode:    mode,
		meta:    meta,
		reward:  map[session.SlotID]int64{},
	}
}

// Err reports the first write error. A recording failure never interrupts
// the session (the record is an output, not a dependency); callers that
// require replay_complete integrity must check Err after the run.
func (w *Writer[W, A, S]) Err() error { return w.err }

var _ session.Recorder[int, int, int] = (*Writer[int, int, int])(nil)

// EpisodeStarted writes the header row to every stream.
func (w *Writer[W, A, S]) EpisodeStarted(start session.EpisodeStart) {
	id := w.meta.EpisodeID
	if id == "" {
		id = start.SessionID
	}
	for _, s := range []struct {
		name string
		dst  interface{ Write([]byte) (int, error) }
	}{
		{"decisions", w.streams.Decisions},
		{"events", w.streams.Events},
		{"outcomes", w.streams.Outcomes},
		{"world", w.streams.World},
	} {
		w.line(s.dst, Header{
			Stream:            s.name,
			SchemaVersion:     SchemaVersion,
			ProtocolVersion:   w.meta.ProtocolVersion,
			EpisodeID:         id,
			Mode:              w.mode,
			Seed:              start.Seed,
			EvaluationVersion: w.meta.EvaluationVersion,
		})
	}
}

// Observed writes a sight-only decision row.
func (w *Writer[W, A, S]) Observed(tick session.Tick, slot session.SlotID, obs S, sig session.EvaluationSignal) {
	w.decision(tick, slot, obs, nil, sig, 0)
}

// Decided writes a decision row with its action.
func (w *Writer[W, A, S]) Decided(tick session.Tick, slot session.SlotID, obs S, action A, sig session.EvaluationSignal, latencyMicros int64) {
	w.decision(tick, slot, obs, action, sig, latencyMicros)
}

func (w *Writer[W, A, S]) decision(tick session.Tick, slot session.SlotID, obs S, action any, sig session.EvaluationSignal, latency int64) {
	w.reward[slot] += int64(sig.RewardDelta)
	row := Decision{
		Tick:          uint64(tick),
		Slot:          uint16(slot),
		AgentKind:     w.meta.AgentKinds[slot],
		Sight:         w.raw(obs),
		Evaluation:    evaluation(sig),
		LatencyMicros: latency,
	}
	if action != nil {
		row.Action = w.raw(action)
	}
	w.line(w.streams.Decisions, row)
}

// Rejected writes a rejection event.
func (w *Writer[W, A, S]) Rejected(tick session.Tick, slot session.SlotID, reason string) {
	w.line(w.streams.Events, Event{Tick: uint64(tick), Kind: "rejected", Slot: uint16(slot), Reason: reason})
}

// Lifecycle writes a lifecycle transition event.
func (w *Writer[W, A, S]) Lifecycle(tick session.Tick, from, to session.State) {
	w.line(w.streams.Events, Event{Tick: uint64(tick), Kind: "lifecycle", From: from.String(), To: to.String()})
}

// WorldCommitted writes the ground-truth row; dropped in AnalysisSampled.
func (w *Writer[W, A, S]) WorldCommitted(tick session.Tick, world *W) {
	if w.streams.World == nil {
		return
	}
	w.line(w.streams.World, World{Tick: uint64(tick), State: w.raw(world)})
}

// Checkpointed writes a checkpoint event; dropped in AnalysisSampled
// (integrity checkpoints belong to replay_complete).
func (w *Writer[W, A, S]) Checkpointed(cp session.Checkpoint) {
	if w.mode != ReplayComplete {
		return
	}
	w.line(w.streams.Events, Event{
		Tick:       uint64(cp.Tick),
		Kind:       "checkpoint",
		WorldHash:  fmt.Sprintf("%016x", cp.WorldHash),
		ActionHash: fmt.Sprintf("%016x", cp.ActionHash),
	})
}

// Ended writes one outcome row per slot.
func (w *Writer[W, A, S]) Ended(tick session.Tick, outcomes []session.SlotOutcome) {
	for _, o := range outcomes {
		w.line(w.streams.Outcomes, Outcome{
			Slot:          uint16(o.Slot),
			Result:        o.Signal.Terminal.String(),
			Reward:        w.reward[o.Slot], // accumulated from the recorded rows
			DurationTicks: uint64(tick),
		})
	}
}

func (w *Writer[W, A, S]) raw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		w.fail(err)
		return json.RawMessage("null")
	}
	return b
}

func (w *Writer[W, A, S]) line(dst interface{ Write([]byte) (int, error) }, v any) {
	if dst == nil {
		return
	}
	if err := writeLine(dst, v); err != nil {
		w.fail(err)
	}
}

func (w *Writer[W, A, S]) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

func evaluation(sig session.EvaluationSignal) Evaluation {
	e := Evaluation{
		Score:       sig.Score,
		Progress:    int64(sig.Progress),
		Evaluation:  int64(sig.Evaluation),
		RewardDelta: int64(sig.RewardDelta),
	}
	if sig.Terminal != session.NotTerminal {
		e.Terminal = sig.Terminal.String()
	}
	return e
}
