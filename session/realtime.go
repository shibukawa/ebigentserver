package session

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

// TickStageRuleSet extends StageRuleSet with a per-tick step for realtime play
// (decision:dual-mode-agent-pacing, realtime_nonblocking). Apply consumes
// validated player inputs; Advance then moves the authoritative world one
// tick forward. ActingSlots is not consulted in realtime pacing — every
// slot may submit every tick and a silent slot simply contributes nothing.
type TickStageRuleSet[S, A, O any] interface {
	StageRuleSet[S, A, O]
	// Advance runs one simulation step after this tick's inputs were
	// applied.
	Advance(state *S)
}

// TimeControl selects how the realtime loop paces (concept:game-time-
// control): a runtime value, never a build tag.
type TimeControl uint8

const (
	// Paced advances at the tuning profile's tick rate on the wall
	// clock — normal play.
	Paced TimeControl = iota
	// Unlimited advances as fast as the machine allows — simulation
	// runs, tests, and replays (concept:training-mode).
	Unlimited
)

// Inbox is one slot's upstream mailbox: transports and local drivers
// Submit actions from any goroutine, and the session drains it at tick
// start. Realtime pacing never blocks on an agent — an empty inbox is
// "no input this tick".
type Inbox[A any] struct {
	mu      sync.Mutex
	queue   []A
	dropped uint64
}

// inboxCap bounds queued inputs per slot; beyond it the oldest input is
// dropped (an honest client at any tick rate stays far below this, and
// flood plausibility checks arrive in Phase 4).
const inboxCap = 64

// Submit queues one action. Safe for concurrent use.
func (in *Inbox[A]) Submit(a A) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.queue) >= inboxCap {
		in.queue = in.queue[1:]
		in.dropped++
	}
	in.queue = append(in.queue, a)
}

// takeNewest empties the inbox and returns the newest action. Stale queued
// inputs are superseded, not replayed — the server-authority late-input
// policy of decision:input-timing-owned-by-sync-mode in its simplest form.
func (in *Inbox[A]) takeNewest() (a A, ok bool) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.queue) == 0 {
		return a, false
	}
	a = in.queue[len(in.queue)-1]
	in.queue = in.queue[:0]
	return a, true
}

// takeAll empties the inbox in arrival order — the command-stream intake,
// where every order matters and per-slot FIFO is the deterministic
// sequence.
func (in *Inbox[A]) takeAll() []A {
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.queue) == 0 {
		return nil
	}
	out := in.queue
	in.queue = nil
	return out
}

// Inbox returns the slot's mailbox for realtime input submission.
func (s *Session[S, A, O]) Inbox(slot SlotID) (*Inbox[A], error) {
	in, ok := s.inboxes[slot]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownSlot, slot)
	}
	return in, nil
}

// RunRealtime moves admitting → running and drives the tick loop until
// the game reaches a terminal position or the context is cancelled
// (operator stop; unfinished slots end Abandoned). The configured rule set
// must implement TickStageRuleSet and Config.Tuning must be declared.
//
// Input intake per tick, in commit order per slot: Config.InputSource
// when set (deterministic schedules — replays and tests), otherwise the
// newest Inbox submission. Downstream state flows through
// Config.Broadcast at the profile's send cadence; the session itself
// never touches a transport (rule:session-independent-of-transport-and-
// agent-kind).
func (s *Session[S, A, O]) RunRealtime(ctx context.Context, tc TimeControl) error {
	game, ok := s.cfg.RuleSet.(TickStageRuleSet[S, A, O])
	if !ok {
		return fmt.Errorf("session: RunRealtime requires the game to implement TickStageRuleSet")
	}
	if s.cfg.Tuning == nil {
		return fmt.Errorf("session: RunRealtime requires a declared TuningProfile (decision:no-framework-tuning-defaults)")
	}
	if err := s.cfg.Tuning.Validate(); err != nil {
		return err
	}
	if s.state != StateAdmitting {
		return fmt.Errorf("%w: RunRealtime in %v", ErrWrongState, s.state)
	}
	for _, slot := range s.slots {
		if _, ok := s.agents[slot]; !ok {
			return fmt.Errorf("%w: %d", ErrSlotEmpty, slot)
		}
	}

	s.world = game.Start(s.cfg.Seed)
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

	sendEvery := s.cfg.Tuning.SendEvery()
	var ticker *time.Ticker
	if tc == Paced {
		ticker = time.NewTicker(time.Second / time.Duration(s.cfg.Tuning.TickRate))
		defer ticker.Stop()
	}

	for {
		signals, done := s.evaluateAll()
		if done {
			return s.drain(signals)
		}
		if ctx.Err() != nil {
			return s.drain(s.abandonRemaining(signals))
		}
		if ticker != nil {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				continue // top of loop drains
			}
		}

		// Intake in commit order (rule:deterministic-tick-commit): slots
		// by id, and within one slot the declared intake policy —
		// newest-only for continuous control, arrival-order FIFO for
		// command streams. Rejected input is dropped; realtime never
		// retries a decision.
		for _, slot := range s.slots {
			for _, action := range s.takeInputs(slot) {
				// Plausibility first (heuristic, authoritative-side
				// only), then legality (deterministic) — the two
				// validator classes of api:action-validator.
				if s.cfg.Plausibility != nil {
					if verr := s.cfg.Plausibility.Plausible(s.tick, slot, action); verr != nil {
						s.rejected(slot, verr.Error())
						continue
					}
				}
				if verr := s.cfg.Validator.Legal(&s.world, slot, action); verr != nil {
					s.rejected(slot, verr.Error())
					continue
				}
				if s.cfg.Recorder != nil {
					s.cfg.Recorder.Decided(s.tick, slot, game.Project(&s.world, slot), action, signals[slot], 0)
				}
				s.commitAction(slot, action)
				game.Apply(&s.world, slot, action)
			}
		}

		game.Advance(&s.world)
		s.tick++ // the single commit point of the tick
		s.recordCommit()

		if s.cfg.Broadcast != nil && s.tick%sendEvery == 0 {
			s.cfg.Broadcast(s.tick, &s.world)
		}
	}
}

// takeInputs collects this tick's inputs for one slot per the declared
// intake policy. An InputSource is polled repeatedly under IntakeAll
// (bounded by the inbox capacity) so scheduled replays can carry several
// actions per slot per tick.
func (s *Session[S, A, O]) takeInputs(slot SlotID) []A {
	all := s.cfg.Tuning != nil && s.cfg.Tuning.InputIntake == IntakeAll
	if s.cfg.InputSource != nil {
		if !all {
			if a, ok := s.cfg.InputSource(s.tick, slot); ok {
				return []A{a}
			}
			return nil
		}
		var out []A
		for len(out) < inboxCap {
			a, ok := s.cfg.InputSource(s.tick, slot)
			if !ok {
				break
			}
			out = append(out, a)
		}
		return out
	}
	if all {
		return s.inboxes[slot].takeAll()
	}
	if a, ok := s.inboxes[slot].takeNewest(); ok {
		return []A{a}
	}
	return nil
}
