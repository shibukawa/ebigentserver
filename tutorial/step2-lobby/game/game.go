// Package game holds the rules of tic-tac-toe, now as a
// api:simulation-interface the framework can host. It still imports no
// engine.
//
// Step 1 promised this step would add methods rather than move code, and
// that is what happened: Legal and Place are the same functions, and
// Start, ActingSlots, Apply, Project and Evaluate are new around them.
// The one thing that did change is where the board lives — it is a wire
// type now (package msg), because a second machine has to see it.
package game

import (
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/tutorial/step2-lobby/msg"
)

// The two seats. X always moves first.
const (
	SlotX session.SlotID = 1
	SlotO session.SlotID = 2
)

// Slots is the game-defined seat set (concept:player-slot).
func Slots() []session.SlotID { return []session.SlotID{SlotX, SlotO} }

// Protocol identifies this game's message schema in every episode header
// and in every handshake. It comes from the generated code, so a change
// that moves one byte on the wire moves this too.
const Protocol = msg.CBORProtocolVersion

// Evaluation versions the scoring in Evaluate.
const Evaluation = 1

// Mark is what occupies one cell. The values are the wire values.
type Mark uint8

const (
	Empty Mark = iota
	X
	O
)

// String renders a mark for the status line.
func (m Mark) String() string {
	switch m {
	case X:
		return "X"
	case O:
		return "O"
	default:
		return "-"
	}
}

// MarkOf is the mark a seat plays.
func MarkOf(slot session.SlotID) Mark {
	switch slot {
	case SlotX:
		return X
	case SlotO:
		return O
	default:
		return Empty
	}
}

// State is concept:world-state, and it is the wire type itself: a board
// this small has nothing to gain from a second shape.
type State = msg.TTTState

// Action is concept:action: claim a cell.
type Action = msg.Move

// Observation is what a seat is allowed to see. Tic-tac-toe has global
// visibility, so it is the same board for both — but it is still a
// distinct type, because the projection is the seam every later game
// changes.
type Observation struct {
	// You is the observing seat and Mark its symbol.
	You  session.SlotID
	Mark Mark
	// Cells is the board.
	Cells [9]Mark
	// Turn is the seat to move, 0 once the game is over.
	Turn session.SlotID
	// Legal lists the cells this seat may take now, so a controller
	// needs no rule engine of its own.
	Legal []uint8
	// Winner and Over report the end.
	Winner session.SlotID
	Over   bool
	// Signal is the seat's data:evaluation-signal, delivered with the
	// observation so every controller has a criterion and not only a
	// legal move set.
	Signal session.EvaluationSignal
}

// Lines are the eight ways to win.
var Lines = [8][3]uint8{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8},
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8},
	{0, 4, 8}, {2, 4, 6},
}

// Simulation is the game's api:simulation-interface. The zero value is
// ready to use.
type Simulation struct{}

var _ session.TickSimulation[State, Action, Observation] = Simulation{}

// Start deals an empty board with X to move. Tic-tac-toe is
// deterministic, so the seed goes unused — it is still recorded, and a
// game that grows randomness derives it from here rather than the clock.
func (Simulation) Start(uint64) State {
	return State{Cells: make([]uint8, 9), Turn: uint16(SlotX)}
}

// ActingSlots returns the seat whose turn it is: strict alternation, one
// decision at a time. It is empty once the game is over, which is legal
// only because every seat then evaluates terminal.
func (Simulation) ActingSlots(s *State) []session.SlotID {
	if s.Over || s.Turn == 0 {
		return nil
	}
	return []session.SlotID{session.SlotID(s.Turn)}
}

// Legal reports whether the seat to move may take cell. It is the same
// judgement step 1 made, now also the one api:action-validator asks.
func Legal(s *State, slot session.SlotID, cell uint8) bool {
	if s.Over || session.SlotID(s.Turn) != slot {
		return false
	}
	return int(cell) < len(s.Cells) && s.Cells[cell] == uint8(Empty)
}

// Apply takes the cell for the seat to move. Legality was settled before
// the call, so this cannot fail.
func (Simulation) Apply(s *State, slot session.SlotID, a Action) {
	if !Legal(s, slot, a.Cell) {
		return
	}
	mark := MarkOf(slot)
	s.Cells[a.Cell] = uint8(mark)
	s.Moves++
	if line, won := lineFor(s, uint8(mark)); won {
		s.Winner, s.Over, s.Turn = uint16(slot), true, 0
		s.Line = []uint8{line[0], line[1], line[2]}
		return
	}
	if int(s.Moves) == len(s.Cells) {
		s.Over, s.Turn = true, 0
		return
	}
	s.Turn = uint16(other(slot))
}

// Advance runs one simulation step, and there is nothing to run: a board
// changes when somebody moves and at no other moment. The method exists
// because the realtime loop asks every game for it, and a turn-based one
// answering "nothing happened" is a real answer rather than a gap — it
// is what lets the same loop, the same recording, and the same link
// serve a board game and a shooter.
func (Simulation) Advance(*State) {}

// Project builds the observation a seat is allowed to see.
func (g Simulation) Project(s *State, slot session.SlotID) Observation {
	obs := Observation{
		You:    slot,
		Mark:   MarkOf(slot),
		Turn:   session.SlotID(s.Turn),
		Winner: session.SlotID(s.Winner),
		Over:   s.Over,
		Signal: g.Evaluate(s, slot),
	}
	for i, v := range s.Cells {
		if i < len(obs.Cells) {
			obs.Cells[i] = Mark(v)
		}
	}
	for cell := range s.Cells {
		if Legal(s, slot, uint8(cell)) {
			obs.Legal = append(obs.Legal, uint8(cell))
		}
	}
	return obs
}

// Evaluate computes the seat's data:evaluation-signal. The session calls
// it; a controller never scores itself.
func (Simulation) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	sig := session.EvaluationSignal{Score: int64(s.Moves)}
	switch {
	case !s.Over:
		sig.Terminal = session.NotTerminal
	case s.Winner == uint16(slot):
		sig.Terminal = session.Win
	case s.Winner == 0:
		sig.Terminal = session.Draw
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

// Config builds one match's session configuration.
func Config(id string, seed uint64) session.Config[State, Action, Observation] {
	tune := Tuning()
	return session.Config[State, Action, Observation]{
		ID:         id,
		Slots:      Slots(),
		Simulation: Simulation{},
		Validator:  Validator{},
		Seed:       seed,
		Tuning:     &tune,
		Canonical:  Canonical,
	}
}

// Tuning is the declared data:session-tuning-profile. A board game has
// no physics to run, so the tick rate only bounds how quickly a click
// becomes a committed move.
func Tuning() session.TuningProfile {
	return session.TuningProfile{
		TickRate: 30, SendRate: 30, HistoryDepth: 8, SnapshotEvery: 30,
	}
}

// Validator is the legality half of api:action-validator. It runs on
// every simulating peer, so it must be deterministic — which it is,
// being the same Legal the rules use.
type Validator struct{}

// Legal refuses a move that is not this seat's to make.
func (Validator) Legal(s *State, slot session.SlotID, a Action) error {
	if Legal(s, slot, a.Cell) {
		return nil
	}
	return errIllegal{cell: a.Cell, slot: slot}
}

type errIllegal struct {
	cell uint8
	slot session.SlotID
}

func (e errIllegal) Error() string {
	return "tictactoe: seat " + MarkOf(e.slot).String() + " cannot take cell " + string('0'+rune(e.cell))
}

// Codec carries the board between the two machines. Snapshot and delta
// both come from the generated code, so neither end can drift from the
// other without the protocol version moving with it.
func Codec() statesync.Codec[State, msg.TTTStateDelta] {
	return statesync.Codec[State, msg.TTTStateDelta]{
		AppendSnapshot: func(dst []byte, s *State) []byte { return s.AppendCBORTo(dst) },
		DecodeSnapshot: func(s *State, data []byte) error { return s.DecodeCBORFrom(data) },
		Diff:           func(base, cur *State) msg.TTTStateDelta { return msg.DiffTTTState(*base, *cur) },
		AppendDelta:    func(dst []byte, d *msg.TTTStateDelta) []byte { return d.AppendCBORTo(dst) },
		DecodeDelta:    func(d *msg.TTTStateDelta, data []byte) error { return d.DecodeCBORFrom(data) },
		ApplyDelta:     msg.ApplyTTTStateDelta,
		// The board is a slice, so a value copy would alias it and
		// every retained baseline would silently be the newest state.
		Clone: func(s *State) State {
			c := *s
			c.Cells = append([]uint8(nil), s.Cells...)
			c.Line = append([]uint8(nil), s.Line...)
			return c
		},
	}
}

// Canonical encodes the state for data:state-checkpoint.
func Canonical(s *State) []byte { return s.AppendCBORTo(nil) }

// EncodeAction and DecodeAction carry data:player-input.
func EncodeAction(dst []byte, a Action) []byte { return append(dst, a.AppendCBORTo(nil)...) }

// DecodeAction reads one input off the wire.
func DecodeAction(b []byte) (Action, error) {
	var a Action
	err := a.DecodeCBORFrom(b)
	return a, err
}

func other(slot session.SlotID) session.SlotID {
	if slot == SlotX {
		return SlotO
	}
	return SlotX
}

func lineFor(s *State, mark uint8) ([3]uint8, bool) {
	for _, line := range Lines {
		if s.Cells[line[0]] == mark && s.Cells[line[1]] == mark && s.Cells[line[2]] == mark {
			return line, true
		}
	}
	return [3]uint8{}, false
}
