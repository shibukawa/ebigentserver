// Package reversi is sample:reversi — the same turn structure as
// tic-tac-toe, but with an action space large enough that controllers
// differ meaningfully.
//
// The new capability (plan.md Phase 2): legal action enumeration is part
// of the observation, so no controller carries a private rule engine — a
// bot picks from Observation.Legal exactly as a human client renders it.
// The flip count each legal move would earn ships as its affordance
// (data:visibility-annotation's affordances field, in the simplest form).
package reversi

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// The two player slots. Black moves first, as the rules of the game say.
const (
	SlotBlack session.SlotID = 1
	SlotWhite session.SlotID = 2
)

// Slots is the game-defined slot set.
func Slots() []session.SlotID { return []session.SlotID{SlotBlack, SlotWhite} }

// Cell is one board cell's content.
type Cell uint8

const (
	Empty Cell = iota
	Black
	White
)

// Board is the 8x8 grid, row-major: cell 0 is a1 (top-left), 63 is h8.
type Board [64]Cell

// State is the game's concept:world-state.
type State struct {
	Board Board
	// Next is the slot to move; 0 once the game is over.
	Next session.SlotID
	// Passes counts consecutive passes; two in a row end the game.
	Passes uint8
	// Over marks a finished game.
	Over bool
}

// Move is the game's concept:action: place a disc, or pass when and only
// when no placement is legal.
type Move struct {
	Cell uint8 `json:"cell"`
	Pass bool  `json:"pass,omitempty"`
}

// LegalMove is one entry of the enumerated legal action set: the move and
// the number of discs it would flip (its affordance, which is all a
// 1-ply controller needs).
type LegalMove struct {
	Move  Move  `json:"move"`
	Flips uint8 `json:"flips"`
}

// Observation is the game's concept:observation, global scope.
type Observation struct {
	You  session.SlotID
	Disc Cell
	// Board is the full position.
	Board Board
	// NextTurn is the slot to move; 0 when the game is over.
	NextTurn session.SlotID
	// Legal enumerates the observing slot's legal moves in stable cell
	// order when it is that slot's turn (a forced pass appears as the
	// single legal move). Empty when it is not.
	Legal []LegalMove
	// Signal is the slot's data:evaluation-signal.
	Signal session.EvaluationSignal
}

// RuleSet implements session.StageRuleSet. The zero value is ready to use.
type RuleSet struct{}

// Start deals the standard opening position; reversi is deterministic, so
// the shared seed is unused.
func (RuleSet) Start(uint64) State {
	s := State{Next: SlotBlack}
	s.Board[27], s.Board[36] = White, White // d4, e5
	s.Board[28], s.Board[35] = Black, Black // e4, d5
	return s
}

// ActingSlots returns the slot to move: strict alternation with forced
// passes represented as actions, so every step has exactly one decision.
func (RuleSet) ActingSlots(s *State) []session.SlotID {
	if s.Over {
		return nil
	}
	return []session.SlotID{s.Next}
}

// Apply plays the (already validated) move.
func (RuleSet) Apply(s *State, slot session.SlotID, m Move) {
	if m.Pass {
		s.Passes++
	} else {
		s.Passes = 0
		disc := disc(slot)
		s.Board[m.Cell] = disc
		for _, dir := range directions {
			flipRay(&s.Board, m.Cell, dir, disc)
		}
	}
	next := opponent(slot)
	if s.Passes >= 2 || full(&s.Board) {
		s.Over = true
		s.Next = 0
		return
	}
	s.Next = next
}

// Project builds a slot's observation, including the enumerated legal
// moves when it is the slot's turn.
func (g RuleSet) Project(s *State, slot session.SlotID) Observation {
	obs := Observation{
		You:      slot,
		Disc:     disc(slot),
		Board:    s.Board,
		NextTurn: s.Next,
		Signal:   g.Evaluate(s, slot),
	}
	if !s.Over && s.Next == slot {
		obs.Legal = LegalMoves(&s.Board, disc(slot))
	}
	return obs
}

// Evaluate computes the slot's data:evaluation-signal: score is the
// slot's disc count, evaluation the signed disc difference, progress the
// board fill.
func (RuleSet) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	own, opp := count(&s.Board, disc(slot)), count(&s.Board, disc(opponent(slot)))
	sig := session.EvaluationSignal{
		Score:      int64(own),
		Evaluation: fixmath.FromInt32(int32(own) - int32(opp)),
		Progress:   fixmath.FromInt32(int32(own) + int32(opp)).Div(fixmath.FromInt32(64)),
	}
	if !s.Over {
		return sig
	}
	switch {
	case own == opp:
		sig.Terminal = session.Draw
	case own > opp:
		sig.Terminal = session.Win
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

// Validator is the legality class of api:action-validator: a placement
// must flip at least one disc, and a pass is legal only when nothing else
// is.
type Validator struct{}

// Legal implements session.ActionValidator.
func (Validator) Legal(s *State, slot session.SlotID, m Move) error {
	if s.Over {
		return fmt.Errorf("reversi: game is over")
	}
	if slot != s.Next {
		return fmt.Errorf("reversi: slot %d moved out of turn (next is %d)", slot, s.Next)
	}
	if m.Pass {
		if !LegalMoves(&s.Board, disc(slot))[0].Move.Pass {
			return fmt.Errorf("reversi: pass with legal moves available")
		}
		return nil
	}
	if m.Cell > 63 {
		return fmt.Errorf("reversi: cell %d out of range", m.Cell)
	}
	if s.Board[m.Cell] != Empty {
		return fmt.Errorf("reversi: cell %d is occupied", m.Cell)
	}
	if countFlips(&s.Board, m.Cell, disc(slot)) == 0 {
		return fmt.Errorf("reversi: cell %d flips nothing", m.Cell)
	}
	return nil
}

// LegalMoves enumerates a disc's legal moves in ascending cell order; a
// position with no flipping placement yields the single forced pass.
func LegalMoves(b *Board, d Cell) []LegalMove {
	var moves []LegalMove
	for cell := uint8(0); cell < 64; cell++ {
		if b[cell] != Empty {
			continue
		}
		if n := countFlips(b, cell, d); n > 0 {
			moves = append(moves, LegalMove{Move: Move{Cell: cell}, Flips: n})
		}
	}
	if len(moves) == 0 {
		return []LegalMove{{Move: Move{Pass: true}}}
	}
	return moves
}

func disc(slot session.SlotID) Cell {
	if slot == SlotBlack {
		return Black
	}
	return White
}

func opponent(slot session.SlotID) session.SlotID {
	if slot == SlotBlack {
		return SlotWhite
	}
	return SlotBlack
}

func other(d Cell) Cell {
	if d == Black {
		return White
	}
	return Black
}

// directions are the 8 ray steps as (dRow, dCol) pairs.
var directions = [8][2]int8{
	{-1, -1}, {-1, 0}, {-1, 1},
	{0, -1}, {0, 1},
	{1, -1}, {1, 0}, {1, 1},
}

// ray walks from cell along dir, returning how many opposing discs lie
// between cell and the first own disc, or 0 when the ray does not close.
func ray(b *Board, cell uint8, dir [2]int8, d Cell) uint8 {
	row, col := int8(cell/8), int8(cell%8)
	var seen uint8
	for {
		row += dir[0]
		col += dir[1]
		if row < 0 || row > 7 || col < 0 || col > 7 {
			return 0
		}
		switch b[uint8(row)*8+uint8(col)] {
		case other(d):
			seen++
		case d:
			return seen
		default:
			return 0
		}
	}
}

func countFlips(b *Board, cell uint8, d Cell) uint8 {
	var n uint8
	for _, dir := range directions {
		n += ray(b, cell, dir, d)
	}
	return n
}

func flipRay(b *Board, cell uint8, dir [2]int8, d Cell) {
	n := ray(b, cell, dir, d)
	row, col := int8(cell/8), int8(cell%8)
	for range n {
		row += dir[0]
		col += dir[1]
		b[uint8(row)*8+uint8(col)] = d
	}
}

func full(b *Board) bool {
	for _, c := range b {
		if c == Empty {
			return false
		}
	}
	return true
}

func count(b *Board, d Cell) uint8 {
	var n uint8
	for _, c := range b {
		if c == d {
			n++
		}
	}
	return n
}
