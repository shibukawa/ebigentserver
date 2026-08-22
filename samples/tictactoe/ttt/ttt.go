// Package ttt is sample:tic-tac-toe — the smallest possible session: two
// slots, strict turns, request and response, terminal conditions. It exists
// to prove that a concept:session exists at all, and it doubles as the
// framework's regression harness (decision:samples-as-test-infrastructure).
//
// Review criterion (decision:no-ai-game-mode): nothing in this package
// branches on what controls a slot. The bot lives in bot.go behind the same
// api:agent-interface a human client uses.
package ttt

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// The two player slots. X always moves first.
const (
	SlotX session.SlotID = 1
	SlotO session.SlotID = 2
)

// Slots is the game-defined slot set (concept:player-slot).
func Slots() []session.SlotID { return []session.SlotID{SlotX, SlotO} }

// Cell is one board cell's content.
type Cell uint8

const (
	Empty Cell = iota
	MarkX
	MarkO
)

// Board is the 3x3 grid, row-major: cell 0 is the top-left, 8 the
// bottom-right.
type Board [9]Cell

// State is the game's concept:world-state.
type State struct {
	Board Board
	// Next is the slot to move; 0 once the game is over.
	Next session.SlotID
	// Winner is the winning slot, or 0 on a draw or an open game.
	Winner session.SlotID
	// Moves counts marks placed.
	Moves uint8
	// Over marks a finished game.
	Over bool
}

// Move is the game's concept:action: place your mark on a cell.
type Move struct {
	Cell uint8 // 0..8
}

// Observation is the game's concept:observation. It is deliberately a
// distinct type from State even though tic-tac-toe has global visibility
// (plan.md seam table): later games change the projection, not the loop.
type Observation struct {
	// You is the observing slot and Mark its symbol.
	You  session.SlotID
	Mark Cell
	// Board is the full board — global scope of concept:visibility-scope.
	Board Board
	// NextTurn is the slot to move; 0 when the game is over.
	NextTurn session.SlotID
	// Signal is the slot's data:evaluation-signal, delivered with the
	// observation so every controller has a criterion.
	Signal session.EvaluationSignal
}

// RuleSet implements session.StageRuleSet. The zero value is ready to use.
type RuleSet struct{}

// Start deals an empty board with X to move.
func (RuleSet) Start(uint64) State { return State{Next: SlotX} }

// ActingSlots returns the slot whose turn it is: strict alternation, one
// decision per step.
func (RuleSet) ActingSlots(s *State) []session.SlotID {
	if s.Over {
		return nil
	}
	return []session.SlotID{s.Next}
}

// Apply places the (already validated) mark and resolves the position.
func (RuleSet) Apply(s *State, slot session.SlotID, m Move) {
	s.Board[m.Cell] = mark(slot)
	s.Moves++
	if winningLine(&s.Board, mark(slot)) {
		s.Winner = slot
		s.Over = true
		s.Next = 0
		return
	}
	if s.Moves == 9 {
		s.Over = true
		s.Next = 0
		return
	}
	s.Next = opponent(slot)
}

// Project builds a slot's observation.
func (g RuleSet) Project(s *State, slot session.SlotID) Observation {
	return Observation{
		You:      slot,
		Mark:     mark(slot),
		Board:    s.Board,
		NextTurn: s.Next,
		Signal:   g.Evaluate(s, slot),
	}
}

// Evaluate computes the slot's data:evaluation-signal. Phase 1 carries the
// terminal outcome plus a trivial progress measure (board fill).
func (RuleSet) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	sig := session.EvaluationSignal{
		Progress: fixmath.FromScaled(int64(s.Moves), 0).Div(fixmath.FromInt32(9)),
	}
	if !s.Over {
		return sig
	}
	switch s.Winner {
	case 0:
		sig.Terminal = session.Draw
	case slot:
		sig.Terminal = session.Win
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

// Validator is the legality class of api:action-validator for tic-tac-toe.
type Validator struct{}

// Legal rejects out-of-range cells, occupied cells, and out-of-turn moves.
func (Validator) Legal(s *State, slot session.SlotID, m Move) error {
	if s.Over {
		return fmt.Errorf("ttt: game is over")
	}
	if slot != s.Next {
		return fmt.Errorf("ttt: slot %d moved out of turn (next is %d)", slot, s.Next)
	}
	if m.Cell > 8 {
		return fmt.Errorf("ttt: cell %d out of range", m.Cell)
	}
	if s.Board[m.Cell] != Empty {
		return fmt.Errorf("ttt: cell %d is occupied", m.Cell)
	}
	return nil
}

func mark(slot session.SlotID) Cell {
	if slot == SlotX {
		return MarkX
	}
	return MarkO
}

func opponent(slot session.SlotID) session.SlotID {
	if slot == SlotX {
		return SlotO
	}
	return SlotX
}

// lines is every winning triple, in a fixed order.
var lines = [8][3]uint8{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // columns
	{0, 4, 8}, {2, 4, 6}, // diagonals
}

func winningLine(b *Board, m Cell) bool {
	for _, l := range lines {
		if b[l[0]] == m && b[l[1]] == m && b[l[2]] == m {
			return true
		}
	}
	return false
}
