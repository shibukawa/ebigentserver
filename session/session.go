// Package session implements concept:session — the runtime that advances
// game state by applying agent actions each step — together with the
// contracts a game and its agents plug into: api:agent-interface,
// api:action-validator, data:evaluation-signal, and data:progress-report.
//
// The session never learns what drives a slot (rule:session-independent-of-
// transport-and-agent-kind): it talks to every controller through the same
// Agent interface, which is what makes human vs bot vs replay a launch-time
// choice instead of a rule variant (decision:no-ai-game-mode).
//
// Phase 1 scope: step pacing only (decision:dual-mode-agent-pacing,
// agent_driven_step), in-process agents, global visibility. Realtime pacing,
// transports, and per-slot visibility arrive in later phases without
// changing these contracts.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"time"
)

// SlotID identifies one concept:player-slot inside a session. IDs are stable
// for the session's whole life and order every commit
// (rule:deterministic-tick-commit). 0 is reserved as the session's own
// entity namespace (decision:owner-namespaced-entity-ids) and is never a
// player slot.
type SlotID uint16

// Tick counts committed simulation steps. In step pacing one decision round
// is one tick.
type Tick uint64

// Game is what a game implements to be hosted (rule:observation-content-
// owned-by-game: every content decision lives here, none in the session).
// S is the game's world state (concept:world-state), A its action type
// (concept:action), O its observation type (concept:observation).
//
// S and O are deliberately distinct type parameters even though a Phase 1
// game may project one into the other almost unchanged: per-slot visibility
// (concept:visibility-scope) later changes Project, never the session loop.
type Game[S, A, O any] interface {
	// Start returns the initial world state. seed is the session's
	// shared RNG seed (rule:shared-rng-seed): a game that needs
	// randomness derives it from here — typically by embedding a
	// fixmath.Rand in its state so the stream advances only through
	// simulation steps and travels with every checkpoint — and never
	// from the wall clock or an unseeded source. Deterministic games
	// ignore it.
	Start(seed uint64) S
	// ActingSlots returns the slots that must decide this step, in any
	// order; the session commits them in SlotID order. Empty means the
	// game has no further decisions, which is only legal once every
	// slot's evaluation is terminal.
	ActingSlots(state *S) []SlotID
	// Apply advances the state by one already-validated action. Legality
	// was established by the ActionValidator; Apply must not fail.
	Apply(state *S, slot SlotID, action A)
	// Project builds the observation a slot is allowed to see. Phase 1
	// games use global scope: every slot sees the same world.
	Project(state *S, slot SlotID) O
	// Evaluate computes the slot's data:evaluation-signal
	// (rule:evaluation-computed-by-session: the session calls this; an
	// agent never scores itself).
	Evaluate(state *S, slot SlotID) EvaluationSignal
}

// Errors returned by lifecycle-guarded methods.
var (
	ErrWrongState  = errors.New("session: operation not allowed in this state")
	ErrUnknownSlot = errors.New("session: unknown slot")
	ErrSlotTaken   = errors.New("session: slot already has an agent")
	ErrSlotEmpty   = errors.New("session: slot has no agent")
)

// Config assembles one session. Zero-value optional fields select safe
// defaults so Phase 1 games only name their game and slots.
type Config[S, A, O any] struct {
	// ID names the session in progress reports.
	ID string
	// Slots is the game-defined slot set (concept:player-slot). Must be
	// non-empty, unique, and must not contain 0.
	Slots []SlotID
	// Game supplies the rules.
	Game Game[S, A, O]
	// Validator judges legality before Apply (api:action-validator).
	// Nil admits every action — the Phase 1 seam default.
	Validator ActionValidator[S, A]
	// Reports receives data:progress-report items. Nil discards them —
	// the Phase 1 seam default (terminal report only).
	Reports ReportSink
	// RetryBudget is how many times one decision may be re-requested
	// after an illegal action before the session aborts (the drop step
	// of api:action-validator's escalation ladder). 0 means the
	// default of 3.
	RetryBudget int
	// Seed is the session's shared RNG seed (rule:shared-rng-seed),
	// passed to Game.Start and recorded in the episode header.
	Seed uint64
	// Recorder receives the episode's record hooks. Nil records
	// nothing.
	Recorder Recorder[S, A, O]
	// Canonical encodes the world state into stable bytes — the
	// canonical input of data:state-checkpoint (the eventual contract
	// is the CBOR world profile encoding; any stable-order encoding
	// serves). Nil disables checkpoints and the world stream.
	Canonical func(state *S) []byte
	// CheckpointEvery emits a checkpoint each time this many ticks
	// commit. 0 means every tick — cheap for turn-based games; tune
	// upward for realtime ones.
	CheckpointEvery Tick
	// Clock returns a monotonic timestamp in microseconds, used only
	// to measure decision latency for the record — measurement
	// metadata, never simulation input. Nil uses the wall clock.
	Clock func() int64
}

// Session hosts one game run. Methods are not safe for concurrent use:
// Phase 1 pacing is a single-threaded step loop.
type Session[S, A, O any] struct {
	cfg        Config[S, A, O]
	slots      []SlotID // sorted; the commit order
	agents     map[SlotID]Agent[O, A]
	state      State
	world      S
	tick       Tick
	seq        uint64 // progress report sequence
	actionHash uint64 // running digest over accepted actions
}

// New validates the configuration and returns a session in StateCreated.
func New[S, A, O any](cfg Config[S, A, O]) (*Session[S, A, O], error) {
	if cfg.Game == nil {
		return nil, errors.New("session: Config.Game is required")
	}
	if len(cfg.Slots) == 0 {
		return nil, errors.New("session: Config.Slots must not be empty")
	}
	slots := slices.Clone(cfg.Slots)
	slices.Sort(slots)
	if slots[0] == 0 {
		return nil, errors.New("session: SlotID 0 is reserved for the session namespace")
	}
	for i := 1; i < len(slots); i++ {
		if slots[i] == slots[i-1] {
			return nil, fmt.Errorf("session: duplicate slot %d", slots[i])
		}
	}
	if cfg.Validator == nil {
		cfg.Validator = AllowAll[S, A]{}
	}
	if cfg.Reports == nil {
		cfg.Reports = Discard{}
	}
	if cfg.RetryBudget == 0 {
		cfg.RetryBudget = 3
	}
	if cfg.CheckpointEvery == 0 {
		cfg.CheckpointEvery = 1
	}
	if cfg.Clock == nil {
		cfg.Clock = wallClock
	}
	return &Session[S, A, O]{
		cfg:    cfg,
		slots:  slots,
		agents: make(map[SlotID]Agent[O, A], len(slots)),
		state:  StateCreated,
	}, nil
}

// State reports the lifecycle state (concept:session-lifecycle).
func (s *Session[S, A, O]) State() State { return s.state }

// Tick reports the number of committed steps.
func (s *Session[S, A, O]) Tick() Tick { return s.tick }

// OpenAdmission moves created → admitting.
func (s *Session[S, A, O]) OpenAdmission() error {
	return s.transition(StateAdmitting)
}

// Admit attaches an agent to an empty slot. Only legal while admitting
// (concept:session-lifecycle forbids admission in created; running-state
// re-admission is a later-phase policy). Joined completes before the
// agent's first Observe.
func (s *Session[S, A, O]) Admit(slot SlotID, agent Agent[O, A]) error {
	if s.state != StateAdmitting {
		return fmt.Errorf("%w: Admit in %v", ErrWrongState, s.state)
	}
	if !slices.Contains(s.slots, slot) {
		return fmt.Errorf("%w: %d", ErrUnknownSlot, slot)
	}
	if _, taken := s.agents[slot]; taken {
		return fmt.Errorf("%w: %d", ErrSlotTaken, slot)
	}
	s.agents[slot] = agent
	agent.Joined(slot)
	return nil
}

// Run moves admitting → running and drives the step loop until the game
// reaches a terminal position, the context is cancelled (operator stop:
// unfinished slots end Abandoned), or an invariant is violated (aborted).
// It returns once the session is in a terminal lifecycle state.
func (s *Session[S, A, O]) Run(ctx context.Context) error {
	if s.state != StateAdmitting {
		return fmt.Errorf("%w: Run in %v", ErrWrongState, s.state)
	}
	for _, slot := range s.slots {
		if _, ok := s.agents[slot]; !ok {
			return fmt.Errorf("%w: %d", ErrSlotEmpty, slot)
		}
	}
	s.world = s.cfg.Game.Start(s.cfg.Seed)
	if s.cfg.Recorder != nil {
		s.cfg.Recorder.EpisodeStarted(EpisodeStart{
			SessionID: s.cfg.ID,
			Seed:      s.cfg.Seed,
			Slots:     slices.Clone(s.slots),
		})
	}
	if err := s.transition(StateRunning); err != nil {
		return err
	}

	for {
		signals, done := s.evaluateAll()
		if done {
			return s.drain(signals)
		}
		if ctx.Err() != nil {
			return s.drain(s.abandonRemaining(signals))
		}
		acting := slices.Clone(s.cfg.Game.ActingSlots(&s.world))
		if len(acting) == 0 {
			return s.abort(fmt.Errorf("session: no acting slots but position is not terminal (tick %d)", s.tick))
		}
		// rule:deterministic-tick-commit — eligibility snapshotted at
		// step start, committed in stable slot order.
		slices.Sort(acting)

		// Every slot observes the tick's opening position before any
		// action of the tick applies; acting slots' projections are
		// retained because the observation as delivered is the record
		// (data:decision-record).
		delivered := s.observeAll(acting, signals)

		for _, slot := range acting {
			action, ok, err := s.decideLegal(ctx, slot, delivered[slot], signals[slot])
			if err != nil {
				return s.abort(err)
			}
			if !ok {
				// No action in step pacing means the controller
				// cannot continue; the session stops as an
				// operator-stop and unfinished slots end
				// Abandoned.
				signals, _ = s.evaluateAll()
				return s.drain(s.abandonRemaining(signals))
			}
			s.commitAction(slot, action)
			s.cfg.Game.Apply(&s.world, slot, action)
		}
		s.tick++ // the single commit point of the step
		s.recordCommit()
	}
}

// commitAction folds one accepted action into the running action digest of
// data:state-checkpoint.
func (s *Session[S, A, O]) commitAction(slot SlotID, action A) {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d:%d:", s.tick, slot)
	if b, err := json.Marshal(action); err == nil {
		h.Write(b)
	} else {
		fmt.Fprintf(h, "!%v", err)
	}
	s.actionHash = s.actionHash*1099511628211 ^ h.Sum64()
}

// recordCommit emits the post-commit ground truth and checkpoint.
func (s *Session[S, A, O]) recordCommit() {
	if s.cfg.Recorder == nil {
		return
	}
	s.cfg.Recorder.WorldCommitted(s.tick, &s.world)
	if s.cfg.Canonical == nil || s.tick%s.cfg.CheckpointEvery != 0 {
		return
	}
	h := fnv.New64a()
	h.Write(s.cfg.Canonical(&s.world))
	s.cfg.Recorder.Checkpointed(Checkpoint{
		Tick:       s.tick,
		WorldHash:  h.Sum64(),
		ActionHash: s.actionHash,
	})
}

// decideLegal obtains one action from a slot's agent, re-requesting up to
// the retry budget when the validator rejects (drop rung of the
// api:action-validator escalation ladder). obs is the observation as
// delivered this tick, retained for the decision record.
func (s *Session[S, A, O]) decideLegal(ctx context.Context, slot SlotID, obs O, sig EvaluationSignal) (action A, ok bool, err error) {
	agent := s.agents[slot]
	for attempt := 0; attempt <= s.cfg.RetryBudget; attempt++ {
		before := s.cfg.Clock()
		action, ok = agent.Decide(ctx)
		latency := s.cfg.Clock() - before
		if !ok {
			return action, false, nil
		}
		if verr := s.cfg.Validator.Legal(&s.world, slot, action); verr == nil {
			if s.cfg.Recorder != nil {
				s.cfg.Recorder.Decided(s.tick, slot, obs, action, sig, latency)
			}
			return action, true, nil
		} else if s.cfg.Recorder != nil {
			s.cfg.Recorder.Rejected(s.tick, slot, verr.Error())
		}
		// Rejected actions never touch the world; the next attempt
		// sees the identical position.
	}
	var zero A
	return zero, false, fmt.Errorf("session: slot %d exhausted retry budget (%d) with illegal actions", slot, s.cfg.RetryBudget)
}

// evaluateAll computes every slot's signal in commit order and reports
// whether all of them are terminal.
func (s *Session[S, A, O]) evaluateAll() (map[SlotID]EvaluationSignal, bool) {
	signals := make(map[SlotID]EvaluationSignal, len(s.slots))
	done := true
	for _, slot := range s.slots {
		sig := s.cfg.Game.Evaluate(&s.world, slot)
		signals[slot] = sig
		if sig.Terminal == NotTerminal {
			done = false
		}
	}
	return signals, done
}

// observeAll delivers each slot's projection in commit order and returns
// the projections of the acting slots for the decision records. The
// evaluation signal travels inside the observation wherever the game's
// Project chooses to put it (data:evaluation-signal is delivered to every
// controller equally); it is also recorded alongside.
//
// Non-acting deliveries are recorded here as observation-only rows;
// acting slots' rows are written by decideLegal once the action is known,
// so each delivery appears exactly once in the decisions stream.
func (s *Session[S, A, O]) observeAll(acting []SlotID, signals map[SlotID]EvaluationSignal) map[SlotID]O {
	delivered := make(map[SlotID]O, len(acting))
	for _, slot := range s.slots {
		obs := s.cfg.Game.Project(&s.world, slot)
		if slices.Contains(acting, slot) {
			delivered[slot] = obs
		} else if s.cfg.Recorder != nil {
			s.cfg.Recorder.Observed(s.tick, slot, obs, signals[slot])
		}
		s.agents[slot].Observe(obs)
	}
	return delivered
}

// abandonRemaining marks every non-terminal slot Abandoned, for operator
// stop and for a controller that returned no action.
func (s *Session[S, A, O]) abandonRemaining(signals map[SlotID]EvaluationSignal) map[SlotID]EvaluationSignal {
	for _, slot := range s.slots {
		sig := signals[slot]
		if sig.Terminal == NotTerminal {
			sig.Terminal = Abandoned
			signals[slot] = sig
		}
	}
	return signals
}

// drain is running → draining → ended: flush progress reports, deliver the
// final observation and result to every agent (agent leave completes before
// the session-end callback per concept:session-lifecycle), then end.
func (s *Session[S, A, O]) drain(signals map[SlotID]EvaluationSignal) error {
	if err := s.transition(StateDraining); err != nil {
		return err
	}
	// Terminal reporting seam (data:progress-report): one report per
	// slot outcome, then the terminal report closing the session record.
	for _, slot := range s.slots {
		if err := s.report(Report{Subject: slot, Kind: "slot_outcome", Outcome: signals[slot].Terminal}); err != nil {
			return s.abort(err)
		}
	}
	if err := s.report(Report{Kind: "session_ended", Terminal: true}); err != nil {
		return s.abort(err)
	}
	s.observeAll(nil, signals)
	for _, slot := range s.slots {
		s.agents[slot].Ended(Result{State: StateEnded, Signal: signals[slot]})
	}
	if s.cfg.Recorder != nil {
		outcomes := make([]SlotOutcome, 0, len(s.slots))
		for _, slot := range s.slots {
			outcomes = append(outcomes, SlotOutcome{Slot: slot, Signal: signals[slot]})
		}
		s.cfg.Recorder.Ended(s.tick, outcomes)
	}
	return s.transition(StateEnded)
}

// abort is the unrecoverable-failure terminal transition. Agents are told;
// the error is returned to the caller.
func (s *Session[S, A, O]) abort(cause error) error {
	if s.state.Terminal() {
		return cause
	}
	if err := s.transition(StateAborted); err != nil {
		return errors.Join(cause, err)
	}
	for _, slot := range s.slots {
		if agent, ok := s.agents[slot]; ok {
			agent.Ended(Result{State: StateAborted, Signal: EvaluationSignal{Terminal: Abandoned}})
		}
	}
	if s.cfg.Recorder != nil {
		outcomes := make([]SlotOutcome, 0, len(s.slots))
		for _, slot := range s.slots {
			outcomes = append(outcomes, SlotOutcome{Slot: slot, Signal: EvaluationSignal{Terminal: Abandoned}})
		}
		s.cfg.Recorder.Ended(s.tick, outcomes)
	}
	return cause
}

func (s *Session[S, A, O]) report(r Report) error {
	s.seq++
	r.SessionID = s.cfg.ID
	r.Seq = s.seq
	r.Tick = s.tick
	return s.cfg.Reports.Report(r)
}

func (s *Session[S, A, O]) transition(to State) error {
	if !s.state.CanTransition(to) {
		return fmt.Errorf("%w: %v -> %v", ErrWrongState, s.state, to)
	}
	if s.cfg.Recorder != nil {
		s.cfg.Recorder.Lifecycle(s.tick, s.state, to)
	}
	s.state = to
	return nil
}

func wallClock() int64 { return time.Now().UnixMicro() }
