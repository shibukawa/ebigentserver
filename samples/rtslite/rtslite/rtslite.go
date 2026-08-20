// Package rtslite is sample:rts-lite — hybrid synchronization at scale:
// command streams upstream, large fog-of-war projections downstream. The
// final rung of the sample ladder, combining every earlier capability.
package rtslite

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/samples/rtslite/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/fixmath"
)

type (
	State = msg.RTSState
	Input = msg.Command
)

// UnitsPerPlayer is the starting army.
const UnitsPerPlayer = 32

// SightRange is fog-of-war visibility around each own unit (Chebyshev).
const SightRange = 10

// Slots returns n player slots (2..4), ids 1..n.
func Slots(n int) []session.SlotID {
	out := make([]session.SlotID, n)
	for i := range out {
		out[i] = session.SlotID(i + 1)
	}
	return out
}

// Observation is the session-level observation: the player's fog view.
type Observation struct {
	You        session.SlotID
	View       *msg.PlayerView
	Signal     session.EvaluationSignal
	Annotation session.VisibilityAnnotation
}

// Game implements session.TickGame.
type Game struct {
	Players   int
	TickLimit uint32
}

var _ session.TickGame[State, Input, Observation] = Game{}

// Start deploys each player's army in its own corner block,
// deterministically from the seed (jittered formation).
func (g Game) Start(seed uint64) State {
	rng := fixmath.NewRand(seed | 1)
	s := State{TickLimit: g.TickLimit}
	if s.TickLimit == 0 {
		s.TickLimit = 3600
	}
	corners := [4][2]uint8{{4, 4}, {msg.MapW - 12, msg.MapH - 12}, {msg.MapW - 12, 4}, {4, msg.MapH - 12}}
	for p := 0; p < g.Players; p++ {
		base := corners[p]
		for u := 0; u < UnitsPerPlayer; u++ {
			jx := uint8(rng.Int64n(8))
			jy := uint8(rng.Int64n(8))
			x, y := base[0]+jx, base[1]+jy
			s.Units = append(s.Units, msg.Unit{
				ID: msg.MakeUnitID(uint16(p+1), s.NextSeq),
				X:  x, Y: y, TX: x, TY: y,
				HP: 3, Alive: true,
			})
			s.NextSeq++
		}
	}
	return s
}

// ActingSlots reports players with an army (step-paced use only).
func (g Game) ActingSlots(s *State) []session.SlotID {
	if s.Over {
		return nil
	}
	return Slots(g.Players)
}

// Apply books one validated order: the unit's standing move target.
func (Game) Apply(s *State, slot session.SlotID, in Input) {
	for i := range s.Units {
		u := &s.Units[i]
		if u.ID == in.Unit && u.Alive {
			u.TX, u.TY = in.TargetX, in.TargetY
			return
		}
	}
}

// Advance moves every unit one step toward its target and resolves
// combat, in unit-id order — deterministic because ids are
// owner-namespaced and stable.
func (Game) Advance(s *State) {
	if s.Over {
		return
	}
	for i := range s.Units {
		u := &s.Units[i]
		if !u.Alive {
			continue
		}
		u.X = stepToward(u.X, u.TX)
		u.Y = stepToward(u.Y, u.TY)
	}
	// Combat: each living unit strikes one adjacent enemy (lowest id
	// first); damage lands simultaneously after all strikes are booked.
	damage := map[int]int8{}
	for i := range s.Units {
		u := &s.Units[i]
		if !u.Alive {
			continue
		}
		for j := range s.Units {
			e := &s.Units[j]
			if !e.Alive || msg.OwnerOf(e.ID) == msg.OwnerOf(u.ID) {
				continue
			}
			if chebyshev(u.X, u.Y, e.X, e.Y) <= 1 {
				damage[j]++
				break
			}
		}
	}
	// Apply damage in stable order (map iteration is not deterministic;
	// walk units in slice order instead).
	for j := range s.Units {
		if d, hit := damage[j]; hit {
			u := &s.Units[j]
			u.HP -= d
			if u.HP <= 0 {
				u.Alive = false
			}
		}
	}
	s.Tick++

	// Standing counts and the end condition.
	survivors := map[uint16]int{}
	for _, u := range s.Units {
		if u.Alive {
			survivors[msg.OwnerOf(u.ID)]++
		}
	}
	if len(survivors) <= 1 || uint32(s.Tick) >= s.TickLimit {
		s.Over = true
		if len(survivors) == 1 {
			for owner := range survivors {
				s.Winner = owner
			}
		}
	}
}

func stepToward(v, target uint8) uint8 {
	if v < target {
		return v + 1
	}
	if v > target {
		return v - 1
	}
	return v
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

// ProjectPlayer is the fog-of-war predicate (term:fog-of-war as a
// visibility predicate of concept:agent-view): own units in full, enemy
// units only inside sight of some own unit. Everything else never
// serializes.
func ProjectPlayer(s *State, slot session.SlotID) msg.PlayerView {
	v := msg.PlayerView{Tick: s.Tick, You: uint16(slot), Over: s.Over, Winner: s.Winner}
	for _, u := range s.Units {
		if msg.OwnerOf(u.ID) == uint16(slot) {
			if u.Alive {
				v.Own = append(v.Own, u)
				v.OwnAlive++
			}
			continue
		}
		if u.Alive {
			v.EnemyAlive++
		}
	}
	for _, e := range s.Units {
		if !e.Alive || msg.OwnerOf(e.ID) == uint16(slot) {
			continue
		}
		for _, own := range v.Own {
			if chebyshev(own.X, own.Y, e.X, e.Y) <= SightRange {
				v.Visible = append(v.Visible, msg.Glimpse{ID: e.ID, X: e.X, Y: e.Y, Owner: msg.OwnerOf(e.ID)})
				break
			}
		}
	}
	return v
}

// Project builds the session observation.
func (g Game) Project(s *State, slot session.SlotID) Observation {
	v := ProjectPlayer(s, slot)
	return Observation{
		You: slot, View: &v, Signal: g.Evaluate(s, slot),
		Annotation: Annotation(&v),
	}
}

// Annotation declares the fog projection (data:visibility-annotation).
func Annotation(v *msg.PlayerView) session.VisibilityAnnotation {
	ann := session.VisibilityAnnotation{
		Scope:           "self",
		Schema:          "rtslite.PlayerView.v1",
		Affordances:     []string{"move_order"},
		EvaluationScope: "scoped",
		Derived:         []string{"enemy_alive_total"},
	}
	for _, u := range v.Own {
		ann.VisibleEntities = append(ann.VisibleEntities, u.ID)
	}
	for _, e := range v.Visible {
		ann.VisibleEntities = append(ann.VisibleEntities, e.ID)
	}
	return ann
}

// Evaluate is scoped to view-visible facts: army sizes are announced in
// this game, positions are not (rule:evaluation-respects-visibility-scope).
func (g Game) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	own, enemies := 0, 0
	for _, u := range s.Units {
		if !u.Alive {
			continue
		}
		if msg.OwnerOf(u.ID) == uint16(slot) {
			own++
		} else {
			enemies++
		}
	}
	sig := session.EvaluationSignal{
		Score:      int64(own),
		Evaluation: fixmath.FromInt32(int32(own - enemies)),
		Progress:   fixmath.FromInt32(int32(s.Tick)).Div(fixmath.FromInt32(int32(s.TickLimit))),
	}
	if !s.Over {
		return sig
	}
	switch {
	case s.Winner == uint16(slot):
		sig.Terminal = session.Win
	case s.Winner == 0:
		sig.Terminal = session.Draw
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

// Validator enforces command legality: you order only your own living
// units, onto the map.
type Validator struct{}

// Legal implements session.ActionValidator.
func (Validator) Legal(s *State, slot session.SlotID, in Input) error {
	if s.Over {
		return fmt.Errorf("rtslite: game is over")
	}
	if msg.OwnerOf(in.Unit) != uint16(slot) {
		return fmt.Errorf("rtslite: unit %d belongs to slot %d", in.Unit, msg.OwnerOf(in.Unit))
	}
	if in.TargetX >= msg.MapW || in.TargetY >= msg.MapH {
		return fmt.Errorf("rtslite: target (%d,%d) off the map", in.TargetX, in.TargetY)
	}
	for _, u := range s.Units {
		if u.ID == in.Unit {
			if !u.Alive {
				return fmt.Errorf("rtslite: unit %d is dead", in.Unit)
			}
			return nil
		}
	}
	return fmt.Errorf("rtslite: unit %d does not exist", in.Unit)
}

// ViewCodec wires the generated player-view codec.
func ViewCodec() statesync.Codec[msg.PlayerView, msg.PlayerViewDelta] {
	return statesync.Codec[msg.PlayerView, msg.PlayerViewDelta]{
		AppendSnapshot: func(dst []byte, v *msg.PlayerView) []byte { return v.AppendCBORTo(dst) },
		DecodeSnapshot: func(v *msg.PlayerView, data []byte) error { return v.DecodeCBORFrom(data) },
		Diff: func(base, cur *msg.PlayerView) msg.PlayerViewDelta {
			return msg.DiffPlayerView(*base, *cur)
		},
		AppendDelta: func(dst []byte, d *msg.PlayerViewDelta) []byte { return d.AppendCBORTo(dst) },
		DecodeDelta: func(d *msg.PlayerViewDelta, data []byte) error { return d.DecodeCBORFrom(data) },
		ApplyDelta:  msg.ApplyPlayerViewDelta,
		Clone: func(v *msg.PlayerView) msg.PlayerView {
			c := *v
			c.Own = append([]msg.Unit(nil), v.Own...)
			c.Visible = append([]msg.Glimpse(nil), v.Visible...)
			return c
		},
	}
}

// MakeSender serves every seat its fog projection.
func MakeSender(tuning session.TuningProfile) func(slot session.SlotID, role string) (statesync.ViewSender[State], error) {
	return func(slot session.SlotID, role string) (statesync.ViewSender[State], error) {
		return statesync.NewProjectedSender(ViewCodec(), tuning, func(s *State) msg.PlayerView {
			return ProjectPlayer(s, slot)
		})
	}
}

// Canonical encodes the full state for data:state-checkpoint.
func Canonical(s *State) []byte { return s.AppendCBORTo(nil) }
