package ttt

import (
	"context"

	"github.com/shibukawa/ebigentserver/session"
)

// Bot is the trivial deterministic bot: it takes the lowest-numbered empty
// cell. It is present to prove decision:no-ai-game-mode from the first
// sample — it sits behind the exact api:agent-interface a human does, and
// the game cannot tell.
type Bot struct {
	last Observation
}

var _ session.Agent[Observation, Move] = (*Bot)(nil)

// Guest records nothing; the bot learns its slot from observations.
func (*Bot) Guest(session.SlotID) {}

// Observe retains the latest observation; policy runs in Decide.
func (b *Bot) Observe(obs Observation) { b.last = obs }

// Decide takes the first empty cell of the last observed board.
func (b *Bot) Decide(context.Context) (Move, bool) {
	for cell := uint8(0); cell < 9; cell++ {
		if b.last.Board[cell] == Empty {
			return Move{Cell: cell}, true
		}
	}
	return Move{}, false
}

// Ended does nothing.
func (*Bot) Ended(session.Result) {}

// Script plays a fixed move list and then returns no action. Tests use it
// as the scripted stand-in for a human: same interface, same seat
// (decision:samples-as-test-infrastructure).
type Script struct {
	// Moves is consumed front to back, one per Decide call, so a
	// rejected move naturally falls through to the next scripted one.
	Moves []Move
	// Results collects what the session reports back.
	Results []session.Result
	// Seen counts observations delivered.
	Seen int
	slot session.SlotID
}

var _ session.Agent[Observation, Move] = (*Script)(nil)

// Guest records the assigned slot.
func (s *Script) Guest(slot session.SlotID) { s.slot = slot }

// Slot returns the slot assigned at admission.
func (s *Script) Slot() session.SlotID { return s.slot }

// Observe counts the delivery.
func (s *Script) Observe(Observation) { s.Seen++ }

// Decide pops the next scripted move.
func (s *Script) Decide(context.Context) (Move, bool) {
	if len(s.Moves) == 0 {
		return Move{}, false
	}
	m := s.Moves[0]
	s.Moves = s.Moves[1:]
	return m, true
}

// Ended records the result.
func (s *Script) Ended(r session.Result) { s.Results = append(s.Results, r) }
