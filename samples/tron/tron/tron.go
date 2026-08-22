// Package tron is sample:tron — realtime scaled from two participants to
// eight, with mixed controllers, spectators, and departures. The game
// itself stays simple; the point of the sample is what happens around it
// when the participant set changes.
package tron

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/samples/tron/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/fixmath"
)

// MaxPlayers bounds the field; a match declares 2..8 slots (1-based ids).
const MaxPlayers = 8

// MaxTicks ends a stalemate as a draw among the living.
const MaxTicks = 4096

// State, Input aliases: the authoritative world is the wire shape.
type (
	State = msg.TronState
	Input = msg.TurnInput
)

// Directions.
const (
	DirUp uint8 = iota
	DirRight
	DirDown
	DirLeft
)

// Observation is the game's concept:observation: global scope.
type Observation struct {
	You    session.SlotID
	State  State
	Signal session.EvaluationSignal
}

// Simulation implements session.TickSimulation for a fixed slot set.
type Simulation struct {
	// SlotIDs is the participating slot set, 1..MaxPlayers.
	SlotIDs []session.SlotID
}

var _ session.TickSimulation[State, Input, Observation] = Simulation{}

// spawns places cycles evenly on the horizontal midline-ish rows facing
// the field center, deterministic in slot order.
func (g Simulation) Start(uint64) State {
	s := State{Alive: uint8(len(g.SlotIDs))}
	for i, slot := range g.SlotIDs {
		x := uint8((i + 1) * msg.GridW / (len(g.SlotIDs) + 1))
		y, dir := uint8(msg.GridH/4), DirDown
		if i%2 == 1 {
			y, dir = uint8(msg.GridH*3/4), DirUp
		}
		s.Players = append(s.Players, msg.Player{ID: uint16(slot), X: x, Y: y, Dir: dir, Alive: true})
	}
	return s
}

// ActingSlots reports the living slots (unused in realtime pacing).
func (g Simulation) ActingSlots(s *State) []session.SlotID {
	if s.Over {
		return nil
	}
	var out []session.SlotID
	for _, p := range s.Players {
		if p.Alive {
			out = append(out, session.SlotID(p.ID))
		}
	}
	return out
}

// Apply turns the (already validated) cycle.
func (Simulation) Apply(s *State, slot session.SlotID, in Input) {
	if p := player(s, slot); p != nil && p.Alive {
		p.Dir = in.Dir
	}
}

// Advance moves every living cycle one cell in slot order, growing trails
// and resolving crashes (rule:deterministic-tick-commit's stable order is
// what makes sequential resolution reproducible).
func (Simulation) Advance(s *State) {
	if s.Over {
		return
	}
	for i := range s.Players {
		p := &s.Players[i]
		if !p.Alive {
			continue
		}
		nx, ny, ok := step(p.X, p.Y, p.Dir)
		if !ok || blocked(s, nx, ny) {
			p.Alive = false
			p.DeathTick = uint32(s.Tick)
			s.Alive--
			continue
		}
		s.Trail = append(s.Trail, msg.TrailCell{ID: s.NextTrail, X: p.X, Y: p.Y, Owner: p.ID})
		s.NextTrail++
		p.X, p.Y = nx, ny
	}
	s.Tick++
	if s.Alive <= 1 || s.Tick >= MaxTicks {
		s.Over = true
		if s.Alive == 1 {
			for _, p := range s.Players {
				if p.Alive {
					s.Winner = p.ID
				}
			}
		}
	}
}

// Project builds a slot's observation.
func (g Simulation) Project(s *State, slot session.SlotID) Observation {
	return Observation{You: slot, State: *s, Signal: g.Evaluate(s, slot)}
}

// Evaluate: score is survival ticks; terminal win for the sole survivor,
// draw for mutual crashes and timeouts, lose otherwise.
func (Simulation) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	p := player(s, slot)
	if p == nil {
		return session.EvaluationSignal{}
	}
	survived := int64(s.Tick)
	if !p.Alive {
		survived = int64(p.DeathTick)
	}
	sig := session.EvaluationSignal{
		Score:    survived,
		Progress: fixmath.FromInt32(int32(s.Tick)).Div(fixmath.FromInt32(MaxTicks)),
	}
	if !s.Over {
		return sig
	}
	switch {
	case s.Winner == uint16(slot):
		sig.Terminal = session.Win
	case s.Winner == 0 && p.Alive:
		sig.Terminal = session.Draw
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

// Validator is the legality class: direction in range and no 180-degree
// reversal (a cycle cannot drive back through itself).
type Validator struct{}

// Legal implements session.ActionValidator.
func (Validator) Legal(s *State, slot session.SlotID, in Input) error {
	if s.Over {
		return fmt.Errorf("tron: game is over")
	}
	if in.Dir > 3 {
		return fmt.Errorf("tron: direction %d out of range", in.Dir)
	}
	p := player(s, slot)
	if p == nil {
		return fmt.Errorf("tron: slot %d has no cycle", slot)
	}
	if !p.Alive {
		return fmt.Errorf("tron: slot %d is dead", slot)
	}
	if in.Dir == opposite(p.Dir) {
		return fmt.Errorf("tron: reversal from %d to %d", p.Dir, in.Dir)
	}
	return nil
}

// Plausibility is the heuristic class of api:action-validator: an honest
// client cannot stamp inputs far in the future. Authoritative-side only;
// free to be a heuristic because rejecting here never touches simulation.
type Plausibility struct {
	// FutureWindow is how many ticks ahead an input stamp may run
	// (clock estimation slack).
	FutureWindow uint32
}

// Plausible implements session.PlausibilityValidator.
func (p Plausibility) Plausible(tick session.Tick, slot session.SlotID, in Input) error {
	if uint64(in.Tick) > uint64(tick)+uint64(p.FutureWindow) {
		return fmt.Errorf("tron: input stamped %d ticks in the future", uint64(in.Tick)-uint64(tick))
	}
	return nil
}

// Codec wires the generated functions into statesync. TronState holds
// slices, so retention needs a real deep copy.
func Codec() statesync.Codec[State, msg.TronStateDelta] {
	return statesync.Codec[State, msg.TronStateDelta]{
		AppendSnapshot: func(dst []byte, s *State) []byte { return s.AppendCBORTo(dst) },
		DecodeSnapshot: func(s *State, data []byte) error { return s.DecodeCBORFrom(data) },
		Diff: func(base, cur *State) msg.TronStateDelta {
			return msg.DiffTronState(*base, *cur)
		},
		AppendDelta: func(dst []byte, d *msg.TronStateDelta) []byte { return d.AppendCBORTo(dst) },
		DecodeDelta: func(d *msg.TronStateDelta, data []byte) error { return d.DecodeCBORFrom(data) },
		ApplyDelta:  msg.ApplyTronStateDelta,
		Clone: func(s *State) State {
			c := *s
			c.Players = append([]msg.Player(nil), s.Players...)
			c.Trail = append([]msg.TrailCell(nil), s.Trail...)
			return c
		},
	}
}

// Canonical encodes the state for data:state-checkpoint.
func Canonical(s *State) []byte { return s.AppendCBORTo(nil) }

func player(s *State, slot session.SlotID) *msg.Player {
	for i := range s.Players {
		if s.Players[i].ID == uint16(slot) {
			return &s.Players[i]
		}
	}
	return nil
}

func opposite(dir uint8) uint8 { return (dir + 2) % 4 }

func step(x, y, dir uint8) (uint8, uint8, bool) {
	switch dir {
	case DirUp:
		if y == 0 {
			return 0, 0, false
		}
		return x, y - 1, true
	case DirDown:
		if y >= msg.GridH-1 {
			return 0, 0, false
		}
		return x, y + 1, true
	case DirLeft:
		if x == 0 {
			return 0, 0, false
		}
		return x - 1, y, true
	default: // DirRight
		if x >= msg.GridW-1 {
			return 0, 0, false
		}
		return x + 1, y, true
	}
}

// blocked reports whether a cell holds trail or another head.
func blocked(s *State, x, y uint8) bool {
	for i := range s.Trail {
		if s.Trail[i].X == x && s.Trail[i].Y == y {
			return true
		}
	}
	for i := range s.Players {
		if s.Players[i].Alive && s.Players[i].X == x && s.Players[i].Y == y {
			return true
		}
	}
	return false
}
