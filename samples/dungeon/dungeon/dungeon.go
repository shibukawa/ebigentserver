// Package dungeon merges sample:cooperative-maze and sample:dungeon-master
// into one Phase 5 sample: one dungeon master who sees everything versus a
// party of role-differentiated adventurers who see almost nothing — self,
// team, and role scopes all in play (concept:visibility-scope), with
// projections that differ in kind between the two sides.
package dungeon

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/samples/dungeon/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// SlotDM runs the dungeon; slots 2..5 are the party in role order
// (scout, engineer, carrier, navigator).
const SlotDM session.SlotID = 1

// Slots returns the DM plus n adventurers (n = 1..4).
func Slots(adventurers int) []session.SlotID {
	out := []session.SlotID{SlotDM}
	for i := 0; i < adventurers; i++ {
		out = append(out, session.SlotID(2+i))
	}
	return out
}

// RoleOf maps a slot to its role.
func RoleOf(slot session.SlotID) uint8 {
	if slot == SlotDM {
		return msg.RoleDM
	}
	return uint8(msg.RoleScout + uint8(slot-2))
}

type (
	State = msg.DungeonState
	Input = msg.ActionInput
)

// Sight radii (Chebyshev): the scout sees wider — role-driven content
// (rule:observation-content-owned-by-game).
const (
	sightDefault = 2
	sightScout   = 4
)

const startHP = 2

// Observation is the session-level observation: one type for the session
// generics, carrying whichever view the slot's role earns — the payloads
// differ in kind. The annotation is the game's explicit visibility
// declaration (data:visibility-annotation), recorded with every decision.
type Observation struct {
	You        session.SlotID
	Role       uint8
	DM         *msg.DMView
	Adventurer *msg.AdventurerView
	Signal     session.EvaluationSignal
	Annotation session.VisibilityAnnotation
}

// Simulation implements session.TickSimulation.
type Simulation struct {
	// Adventurers is the party size (1..4).
	Adventurers int
	// TickLimit ends the crawl with a DM win; 0 means 3600.
	TickLimit uint32
}

var _ session.TickSimulation[State, Input, Observation] = Simulation{}

// Start builds the maze from the shared seed (rule:shared-rng-seed): the
// same seed reproduces the same dungeon on every peer and every replay.
func (g Simulation) Start(seed uint64) State {
	rng := fixmath.NewRand(seed | 1)
	s := State{
		Walls:      make([]uint8, msg.BitmapLen),
		Explored:   make([]uint8, msg.BitmapLen),
		TrapBudget: 8,
		TickLimit:  g.TickLimit,
	}
	if s.TickLimit == 0 {
		s.TickLimit = 3600
	}
	// Border walls plus scattered rocks.
	for x := uint8(0); x < msg.GridW; x++ {
		setBit(s.Walls, x, 0)
		setBit(s.Walls, x, msg.GridH-1)
	}
	for y := uint8(0); y < msg.GridH; y++ {
		setBit(s.Walls, 0, y)
		setBit(s.Walls, msg.GridW-1, y)
	}
	for i := 0; i < msg.GridW*msg.GridH*15/100; i++ {
		x := uint8(rng.Int64n(msg.GridW-2)) + 1
		y := uint8(rng.Int64n(msg.GridH-2)) + 1
		setBit(s.Walls, x, y)
	}
	// Spawns in the top-left room, exit bottom-right, treasure mid-map;
	// clear those cells and a corridor cross to keep the maze winnable.
	s.ExitX, s.ExitY = msg.GridW-3, msg.GridH-3
	s.TreasureX, s.TreasureY = msg.GridW/2, msg.GridH/2
	for i := 0; i < g.Adventurers; i++ {
		a := msg.Adventurer{
			ID: uint16(2 + i), X: uint8(2 + i), Y: 2,
			Role: msg.RoleScout + uint8(i), HP: startHP, Alive: true,
		}
		clearBit(s.Walls, a.X, a.Y)
		s.Adventurers = append(s.Adventurers, a)
	}
	clearBit(s.Walls, s.ExitX, s.ExitY)
	clearBit(s.Walls, s.TreasureX, s.TreasureY)
	for x := uint8(1); x < msg.GridW-1; x++ {
		clearBit(s.Walls, x, msg.GridH/2)
	}
	for y := uint8(1); y < msg.GridH-1; y++ {
		clearBit(s.Walls, msg.GridW/2, y)
	}
	markSight(&s)
	return s
}

// ActingSlots reports whoever can still act (step-paced use only).
func (g Simulation) ActingSlots(s *State) []session.SlotID {
	if s.Over {
		return nil
	}
	out := []session.SlotID{SlotDM}
	for _, a := range s.Adventurers {
		if a.Alive {
			out = append(out, session.SlotID(a.ID))
		}
	}
	return out
}

// Apply performs one validated action.
func (Simulation) Apply(s *State, slot session.SlotID, in Input) {
	if slot == SlotDM {
		switch in.Kind {
		case msg.ActPlaceTrap:
			s.Traps = append(s.Traps, msg.Trap{
				ID: uint32(len(s.Traps)) + 1, X: in.X, Y: in.Y, Armed: true,
			})
			s.TrapBudget--
		}
		return
	}
	a := adventurer(s, slot)
	if a == nil || !a.Alive {
		return
	}
	switch in.Kind {
	case msg.ActMove:
		nx, ny, ok := step(a.X, a.Y, in.Dir)
		if !ok || bit(s.Walls, nx, ny) {
			return
		}
		a.X, a.Y = nx, ny
		// Springing a trap reveals and disarms it, at a price.
		for i := range s.Traps {
			tr := &s.Traps[i]
			if tr.Armed && tr.X == nx && tr.Y == ny {
				tr.Armed = false
				tr.Discovered = true
				a.HP--
				if a.HP <= 0 {
					a.Alive = false
				}
			}
		}
		// Only the carrier lifts the treasure.
		if a.Role == msg.RoleCarrier && a.Alive && !treasureHeld(s) &&
			nx == s.TreasureX && ny == s.TreasureY {
			a.Carrying = true
		}
	case msg.ActDisarm:
		// Engineer only; adjacency and discovery were validated.
		for i := range s.Traps {
			tr := &s.Traps[i]
			if tr.X == in.X && tr.Y == in.Y {
				tr.Armed = false
				tr.Discovered = true
			}
		}
	}
}

// Advance runs one tick: sight and discovery update, then the end
// conditions.
func (Simulation) Advance(s *State) {
	if s.Over {
		return
	}
	markSight(s)
	s.Tick++

	alive := 0
	for _, a := range s.Adventurers {
		if a.Alive {
			alive++
			if a.Carrying && a.X == s.ExitX && a.Y == s.ExitY {
				s.Over = true
				s.Winner = 1 // party
				return
			}
		}
	}
	if alive == 0 || uint32(s.Tick) >= s.TickLimit {
		s.Over = true
		s.Winner = 2 // dungeon master
	}
}

// markSight extends the team's explored bitmap and discovers adjacent
// traps (team scope accumulation lives in the world state, so it is
// simulation-deterministic and checkpointed).
func markSight(s *State) {
	for _, a := range s.Adventurers {
		if !a.Alive {
			continue
		}
		radius := int16(sightDefault)
		if a.Role == msg.RoleScout {
			radius = sightScout
		}
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				x, y := int16(a.X)+dx, int16(a.Y)+dy
				if x >= 0 && y >= 0 && x < msg.GridW && y < msg.GridH {
					setBit(s.Explored, uint8(x), uint8(y))
				}
			}
		}
		for i := range s.Traps {
			tr := &s.Traps[i]
			if !tr.Discovered && chebyshev(a.X, a.Y, tr.X, tr.Y) <= 1 {
				tr.Discovered = true
			}
		}
	}
}

func treasureHeld(s *State) bool {
	for _, a := range s.Adventurers {
		if a.Carrying {
			return true
		}
	}
	return false
}

func adventurer(s *State, slot session.SlotID) *msg.Adventurer {
	for i := range s.Adventurers {
		if s.Adventurers[i].ID == uint16(slot) {
			return &s.Adventurers[i]
		}
	}
	return nil
}

func chebyshev(ax, ay, bx, by uint8) int16 {
	dx, dy := int16(ax)-int16(bx), int16(ay)-int16(by)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

func step(x, y, dir uint8) (uint8, uint8, bool) {
	switch dir {
	case 0:
		if y == 0 {
			return 0, 0, false
		}
		return x, y - 1, true
	case 1:
		if x >= msg.GridW-1 {
			return 0, 0, false
		}
		return x + 1, y, true
	case 2:
		if y >= msg.GridH-1 {
			return 0, 0, false
		}
		return x, y + 1, true
	default:
		if x == 0 {
			return 0, 0, false
		}
		return x - 1, y, true
	}
}

// Bit helpers over the row-major grid bitmaps.
func idx(x, y uint8) (int, uint8) { i := int(y)*msg.GridW + int(x); return i / 8, uint8(i % 8) }

func bit(b []uint8, x, y uint8) bool { i, o := idx(x, y); return b[i]&(1<<o) != 0 }
func setBit(b []uint8, x, y uint8)   { i, o := idx(x, y); b[i] |= 1 << o }
func clearBit(b []uint8, x, y uint8) { i, o := idx(x, y); b[i] &^= 1 << o }

// Bit re-exported for tests and bots.
func Bit(b []uint8, x, y uint8) bool { return bit(b, x, y) }

// Validator is the legality class of api:action-validator.
type Validator struct{}

// Legal enforces role-scoped actions: only the DM places traps, only the
// engineer disarms, only living adventurers move.
func (Validator) Legal(s *State, slot session.SlotID, in Input) error {
	if s.Over {
		return fmt.Errorf("dungeon: game is over")
	}
	if slot == SlotDM {
		if in.Kind != msg.ActPlaceTrap {
			return fmt.Errorf("dungeon: the dungeon master only places traps")
		}
		if s.TrapBudget == 0 {
			return fmt.Errorf("dungeon: trap budget exhausted")
		}
		if in.X >= msg.GridW || in.Y >= msg.GridH || bit(s.Walls, in.X, in.Y) {
			return fmt.Errorf("dungeon: trap target (%d,%d) blocked", in.X, in.Y)
		}
		if in.X == s.ExitX && in.Y == s.ExitY || in.X == s.TreasureX && in.Y == s.TreasureY {
			return fmt.Errorf("dungeon: trap may not cover the exit or treasure")
		}
		for _, a := range s.Adventurers {
			if a.Alive && a.X == in.X && a.Y == in.Y {
				return fmt.Errorf("dungeon: trap under an adventurer")
			}
		}
		return nil
	}
	a := adventurer(s, slot)
	if a == nil {
		return fmt.Errorf("dungeon: slot %d has no adventurer", slot)
	}
	if !a.Alive {
		return fmt.Errorf("dungeon: slot %d is dead", slot)
	}
	switch in.Kind {
	case msg.ActMove:
		if in.Dir > 3 {
			return fmt.Errorf("dungeon: direction %d out of range", in.Dir)
		}
		return nil
	case msg.ActDisarm:
		if a.Role != msg.RoleEngineer {
			return fmt.Errorf("dungeon: only the engineer disarms")
		}
		if chebyshev(a.X, a.Y, in.X, in.Y) > 1 {
			return fmt.Errorf("dungeon: disarm target not adjacent")
		}
		for _, tr := range s.Traps {
			if tr.X == in.X && tr.Y == in.Y && tr.Discovered {
				return nil
			}
		}
		return fmt.Errorf("dungeon: no discovered trap at (%d,%d)", in.X, in.Y)
	default:
		return fmt.Errorf("dungeon: adventurers cannot perform action %d", in.Kind)
	}
}
