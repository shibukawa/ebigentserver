package dungeon

import (
	"github.com/shibukawa/ebigentserver/samples/dungeon/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/fixmath"
)

// ProjectAdventurer builds the party wire view: the visibility predicate
// of rule:observation-content-owned-by-game. Everything absent here is
// absent from the wire — undiscovered traps, unexplored walls, and the
// exit for anyone but the navigator are never encoded.
func ProjectAdventurer(s *State, slot session.SlotID) msg.AdventurerView {
	a := adventurer(s, slot)
	v := msg.AdventurerView{
		Tick:      s.Tick,
		You:       uint16(slot),
		Explored:  append([]uint8(nil), s.Explored...),
		TreasureX: msg.Unknown, TreasureY: msg.Unknown,
		ExitX: msg.Unknown, ExitY: msg.Unknown,
		Over: s.Over, Winner: s.Winner,
	}
	if a != nil {
		v.Role, v.HP, v.X, v.Y, v.Carrying = a.Role, a.HP, a.X, a.Y, a.Carrying
	}
	// Walls masked to explored cells.
	v.KnownWalls = make([]uint8, msg.BitmapLen)
	for i := range v.KnownWalls {
		v.KnownWalls[i] = s.Walls[i] & s.Explored[i]
	}
	// Team scope: every teammate, all the time.
	for _, m := range s.Adventurers {
		v.Party = append(v.Party, msg.PartyMate{
			ID: m.ID, X: m.X, Y: m.Y, Role: m.Role, Alive: m.Alive, Carrying: m.Carrying,
		})
	}
	// Only discovered traps exist for the party.
	for _, tr := range s.Traps {
		if tr.Discovered {
			v.KnownTraps = append(v.KnownTraps, tr)
		}
	}
	// The treasure appears once its cell is explored (or is carried).
	if bit(s.Explored, s.TreasureX, s.TreasureY) {
		v.TreasureX, v.TreasureY = s.TreasureX, s.TreasureY
	}
	// Role scope: the navigator knows the destination.
	if a != nil && a.Role == msg.RoleNavigator {
		v.ExitX, v.ExitY = s.ExitX, s.ExitY
	}
	return v
}

// ProjectDM builds the dungeon master's wire view: the whole map plus
// the party's knowledge (role scope in the other direction).
func ProjectDM(s *State) msg.DMView {
	return msg.DMView{
		Tick:        s.Tick,
		Walls:       append([]uint8(nil), s.Walls...),
		Explored:    append([]uint8(nil), s.Explored...),
		Adventurers: append([]msg.Adventurer(nil), s.Adventurers...),
		Traps:       append([]msg.Trap(nil), s.Traps...),
		TrapBudget:  s.TrapBudget,
		TreasureX:   s.TreasureX, TreasureY: s.TreasureY,
		ExitX: s.ExitX, ExitY: s.ExitY,
		Over: s.Over, Winner: s.Winner,
	}
}

// Project builds the session-level observation for a slot; the recorded
// observation is therefore the same projection the wire carries.
func (g RuleSet) Project(s *State, slot session.SlotID) Observation {
	obs := Observation{You: slot, Role: RoleOf(slot), Signal: g.Evaluate(s, slot)}
	if slot == SlotDM {
		v := ProjectDM(s)
		obs.DM = &v
		obs.Annotation = DMAnnotation(&v)
		return obs
	}
	v := ProjectAdventurer(s, slot)
	obs.Adventurer = &v
	obs.Annotation = AdventurerAnnotation(&v)
	return obs
}

// AdventurerAnnotation declares what the view contains
// (data:visibility-annotation, game-emitted).
func AdventurerAnnotation(v *msg.AdventurerView) session.VisibilityAnnotation {
	ann := session.VisibilityAnnotation{
		Scope:           "team",
		Schema:          "dungeon.AdventurerView.v1",
		Affordances:     []string{"move"},
		EvaluationScope: "scoped",
	}
	for _, m := range v.Party {
		ann.VisibleEntities = append(ann.VisibleEntities, uint32(m.ID))
	}
	for _, tr := range v.KnownTraps {
		ann.VisibleEntities = append(ann.VisibleEntities, 1000+tr.ID)
	}
	switch v.Role {
	case msg.RoleEngineer:
		ann.Affordances = append(ann.Affordances, "disarm")
	case msg.RoleNavigator:
		ann.Scope = "team+role"
		ann.Derived = append(ann.Derived, "exit_position")
	}
	return ann
}

// DMAnnotation declares the dungeon master's privileged projection.
func DMAnnotation(v *msg.DMView) session.VisibilityAnnotation {
	return session.VisibilityAnnotation{
		Scope:           "role",
		Schema:          "dungeon.DMView.v1",
		Affordances:     []string{"place_trap"},
		EvaluationScope: "privileged",
	}
}

// Evaluate computes the slot's signal from what the slot can see
// (rule:evaluation-respects-visibility-scope): the party's numbers derive
// only from team knowledge — exploration, party health, the carry — so
// no hidden trap or unseen DM action ever moves them. The DM's signal is
// privileged, which its annotation declares.
func (g RuleSet) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	explored := 0
	for _, b := range s.Explored {
		for ; b != 0; b &= b - 1 {
			explored++
		}
	}
	alive := 0
	for _, a := range s.Adventurers {
		if a.Alive {
			alive++
		}
	}
	var sig session.EvaluationSignal
	if slot == SlotDM {
		// Privileged: the DM sees everything anyway.
		sig.Score = int64(len(s.Adventurers) - alive)
		sig.Progress = fixmath.FromInt32(int32(s.Tick)).Div(fixmath.FromInt32(int32(s.TickLimit)))
	} else {
		// Scoped: explored cells, live teammates, and the carry are
		// all team-visible facts.
		sig.Score = int64(explored)
		stage := int32(0)
		if treasureHeld(s) {
			stage = 1
		}
		sig.Progress = fixmath.FromInt32(int32(explored)).
			Div(fixmath.FromInt32(msg.GridW * msg.GridH * 2)).
			Add(fixmath.FromInt32(stage).Div(fixmath.FromInt32(2)))
		sig.Evaluation = fixmath.FromInt32(int32(alive))
	}
	if !s.Over {
		return sig
	}
	partyWon := s.Winner == 1
	if (slot == SlotDM) != partyWon {
		sig.Terminal = session.Win
	} else {
		sig.Terminal = session.Lose
	}
	return sig
}

// AdventurerCodec wires the party view codec for statesync.
func AdventurerCodec() statesync.Codec[msg.AdventurerView, msg.AdventurerViewDelta] {
	return statesync.Codec[msg.AdventurerView, msg.AdventurerViewDelta]{
		AppendSnapshot: func(dst []byte, v *msg.AdventurerView) []byte { return v.AppendCBORTo(dst) },
		DecodeSnapshot: func(v *msg.AdventurerView, data []byte) error { return v.DecodeCBORFrom(data) },
		Diff: func(base, cur *msg.AdventurerView) msg.AdventurerViewDelta {
			return msg.DiffAdventurerView(*base, *cur)
		},
		AppendDelta: func(dst []byte, d *msg.AdventurerViewDelta) []byte { return d.AppendCBORTo(dst) },
		DecodeDelta: func(d *msg.AdventurerViewDelta, data []byte) error { return d.DecodeCBORFrom(data) },
		ApplyDelta:  msg.ApplyAdventurerViewDelta,
		Clone: func(v *msg.AdventurerView) msg.AdventurerView {
			c := *v
			c.Explored = append([]uint8(nil), v.Explored...)
			c.KnownWalls = append([]uint8(nil), v.KnownWalls...)
			c.Party = append([]msg.PartyMate(nil), v.Party...)
			c.KnownTraps = append([]msg.Trap(nil), v.KnownTraps...)
			return c
		},
	}
}

// DMCodec wires the dungeon master view codec.
func DMCodec() statesync.Codec[msg.DMView, msg.DMViewDelta] {
	return statesync.Codec[msg.DMView, msg.DMViewDelta]{
		AppendSnapshot: func(dst []byte, v *msg.DMView) []byte { return v.AppendCBORTo(dst) },
		DecodeSnapshot: func(v *msg.DMView, data []byte) error { return v.DecodeCBORFrom(data) },
		Diff: func(base, cur *msg.DMView) msg.DMViewDelta {
			return msg.DiffDMView(*base, *cur)
		},
		AppendDelta: func(dst []byte, d *msg.DMViewDelta) []byte { return d.AppendCBORTo(dst) },
		DecodeDelta: func(d *msg.DMViewDelta, data []byte) error { return d.DecodeCBORFrom(data) },
		ApplyDelta:  msg.ApplyDMViewDelta,
		Clone: func(v *msg.DMView) msg.DMView {
			c := *v
			c.Walls = append([]uint8(nil), v.Walls...)
			c.Explored = append([]uint8(nil), v.Explored...)
			c.Adventurers = append([]msg.Adventurer(nil), v.Adventurers...)
			c.Traps = append([]msg.Trap(nil), v.Traps...)
			return c
		},
	}
}

// MakeSender is the netplay factory: the seat's role selects which
// projection pipeline serves it — the point of the whole phase.
func MakeSender(tuning session.TuningProfile) func(slot session.SlotID, role string) (statesync.ViewSender[State], error) {
	return func(slot session.SlotID, role string) (statesync.ViewSender[State], error) {
		if slot == SlotDM && role != "spectator" {
			return statesync.NewProjectedSender(DMCodec(), tuning, func(s *State) msg.DMView {
				return ProjectDM(s)
			})
		}
		// Spectators ride the adventurer projection of their named
		// seat: wider views are a policy choice this sample declines.
		return statesync.NewProjectedSender(AdventurerCodec(), tuning, func(s *State) msg.AdventurerView {
			return ProjectAdventurer(s, slot)
		})
	}
}

// Canonical encodes the full state for data:state-checkpoint.
func Canonical(s *State) []byte { return s.AppendCBORTo(nil) }
