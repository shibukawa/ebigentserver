// Package game holds the rules of solo: one person is chased across a
// field by two enemies, and survives by staying uncaught until the clock
// runs out.
//
// It is a solo game, and it is on this framework anyway. The reason is
// the enemies. Each one occupies a concept:player-slot and decides
// through api:agent-interface exactly as a remote player would, so every
// enemy decision is recorded into data:episode-log with the observation
// it was made from. That corpus is what
// flow:behavior-tree-synthesis distills into data:behavior-chip, and it
// is the one thing a hand-written enemy loop never produces.
//
// Two of the three hooks of api:tick-hooks therefore run for a
// non-player: intake, where the enemy submits its action, and apply,
// where it receives the world it decided against. Only arbitration is
// central, and in a solo game it is central inside this same process.
//
// Nothing here imports Ebitengine, or any engine at all
// (rule:engine-import-confined-to-client-entry). Rendering and key
// reading live in the client entry point, which is what lets the headless
// entry point exist at all.
package game

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// Field geometry in game units. The renderer scales these to pixels; the
// simulation never knows what a pixel is.
const (
	FieldW = 280
	FieldH = 200
	// ActorR is every actor's radius.
	ActorR = 4
	// CatchR is how close an enemy must get to catch the player.
	CatchR = 7
	// TargetTicks is how long the player must survive. It also bounds
	// the episode, so an unattended run always terminates.
	TargetTicks = 240
)

// Seats is how many concept:player-slot entries these rules declare: one
// person and two enemies, one of each pursuit kind. It is a rule, not a
// setting.
const Seats = 3

// The player slots. Slot 0 is reserved by the session, so slots start at
// 1. Player is the seat a person takes; the rest are enemies, and the
// rules never ask which is which beyond this naming
// (decision:no-ai-game-mode).
const (
	Player session.SlotID = 1
	Enemy1 session.SlotID = 2
	Enemy2 session.SlotID = 3
)

// Slots is the game-defined slot set.
func Slots() []session.SlotID {
	out := make([]session.SlotID, 0, Seats)
	for i := range Seats {
		out = append(out, session.SlotID(i+1))
	}
	return out
}

// IsEnemy reports whether a slot is one of the pursuers.
func IsEnemy(slot session.SlotID) bool { return slot != Player }

// Speeds and distances. Every one is fixed point:
// rule:no-float-in-simulation keeps reals out of anything that has to
// reproduce bit for bit on another machine, which is what replay and
// cross-architecture digests depend on.
//
// The player is faster than any single enemy, so being caught is a
// question of being surrounded rather than outrun. That is what makes the
// enemies' choice of direction worth recording.
var (
	playerSpeed = fixmath.FromInt32(2)
	enemySpeed  = fixmath.FromScaled(5, 2) // 1.25 units per tick
	catchR      = fixmath.FromInt32(CatchR)
	actorR      = fixmath.FromInt32(ActorR)
	fieldW      = fixmath.FromInt32(FieldW)
	fieldH      = fixmath.FromInt32(FieldH)
	// maxDist bounds the field diagonal closely enough to normalize
	// progress without a square root.
	maxDist = fixmath.FromInt32(FieldW + FieldH)
)

// Dir is one of the five things any actor can do with a tick.
type Dir uint8

const (
	// Stay holds position.
	Stay Dir = iota
	Up
	Down
	Left
	Right
	// dirCount bounds the enum for the validator.
	dirCount
)

// String names the direction for logs and the distilled predicates that
// will eventually be written against it.
func (d Dir) String() string {
	switch d {
	case Stay:
		return "stay"
	case Up:
		return "up"
	case Down:
		return "down"
	case Left:
		return "left"
	case Right:
		return "right"
	default:
		return "invalid"
	}
}

// Action is one concept:action: a direction to hold this tick.
type Action struct {
	Move Dir `json:"move"`
}

// Actor is one seat's body on the field.
//
// The coordinates are recorded as their raw fixed-point values rather
// than rounded: the record has to be the number the simulation used, or a
// distilled predicate written against it would not reproduce.
type Actor struct {
	X fixmath.F64 `json:"x"`
	Y fixmath.F64 `json:"y"`
	// Move is the direction held since the last input. Holding rather
	// than clearing means a tick with no input continues what was
	// already happening, which is what a controller expects and what a
	// dropped packet should look like.
	Move Dir `json:"move"`
}

// State is the concept:world-state: everything the session commits.
type State struct {
	Tick  session.Tick
	Actor [Seats]Actor
	// Caught ends the episode in the enemies' favor.
	Caught bool
	// By names the enemy that caught the player, for the record.
	By session.SlotID
	// Rand travels with the state so the opening positions are a
	// function of the seed alone (rule:shared-rng-seed).
	Rand fixmath.Rand
	Over bool
}

// Observation is what one slot may see (concept:observation).
//
// It is a separate type from State on purpose. Every seat sees the whole
// field here, which is the global scope of concept:visibility-scope;
// giving an enemy a limited view later is a change to Project alone, and
// no controller can read around it because none ever receives a State.
type Observation struct {
	You session.SlotID `json:"you"`
	// Self is the observing seat's body.
	Self Actor `json:"self"`
	// Quarry is the player's body — what an enemy is chasing and what
	// the player is protecting. It is the same field for both, because
	// the rules do not branch on who is asking.
	Quarry Actor `json:"quarry"`
	// Others holds every other seat in slot order, so an enemy can see
	// where its companions are and avoid all arriving from one side.
	Others []Actor      `json:"others"`
	Tick   session.Tick `json:"tick"`
	Over   bool         `json:"over"`
	// Signal travels with the observation so every controller has a
	// criterion without asking for one.
	Signal session.EvaluationSignal `json:"signal"`
}

// GapX and GapY are the vector from the observer to the quarry: how far,
// and in which sign, the player lies. They are facts about an observation
// and nothing more — no policy is expressed by measuring a distance.
//
// They exist as named functions because a distilled predicate is written
// against them (data:derived-predicate), and a rule reading
// "GapX(obs) > 0" survives review in a way that the same subtraction
// spelled out inline does not.
func GapX(o Observation) fixmath.F64 { return o.Quarry.X.Sub(o.Self.X) }

// GapY is the vertical half of GapX.
func GapY(o Observation) fixmath.F64 { return o.Quarry.Y.Sub(o.Self.Y) }

// index maps a slot to its array position.
func index(slot session.SlotID) int { return int(slot) - 1 }

// Simulation implements session.TickSimulation: session.Simulation plus Advance, which is
// what a realtime session needs.
type Simulation struct{}

var _ session.TickSimulation[State, Action, Observation] = Simulation{}

// Start returns the opening position, derived from the session's shared
// RNG seed. Two runs with the same seed place the same enemies, which is
// what makes an episode replayable.
func (Simulation) Start(seed uint64) State {
	s := State{Rand: fixmath.NewRand(seed)}
	s.Actor[index(Player)] = Actor{
		X: fieldW.Div(fixmath.FromInt32(2)),
		Y: fieldH.Div(fixmath.FromInt32(2)),
	}
	// Enemies open at the edges, and which edge is drawn from the seed
	// rather than fixed per enemy. That is a balance decision and a
	// corpus decision at once: an enemy that always starts above the
	// quarry never records a reason to move up, and a corpus missing a
	// situation teaches nothing about it
	// (concept:continuous-match-loop, diversity).
	const margin = 12
	for i := 1; i < Seats; i++ {
		x := fixmath.FromInt32(int32(margin + int(s.Rand.Int64n(FieldW-2*margin))))
		y := fixmath.FromInt32(margin)
		if s.Rand.Int64n(2) == 0 {
			y = fixmath.FromInt32(FieldH - margin)
		}
		s.Actor[i] = Actor{X: x, Y: y}
	}
	return s
}

// ActingSlots reports who still has decisions to make. A realtime session
// reads every slot's inbox each tick rather than asking whose turn it is,
// so this mainly answers "is the episode still running".
func (Simulation) ActingSlots(s *State) []session.SlotID {
	if s.Over {
		return nil
	}
	return Slots()
}

// Apply advances the state by one already-validated action. Legality was
// settled by Validator, so Apply cannot fail and must not re-check.
func (Simulation) Apply(s *State, slot session.SlotID, a Action) {
	s.Actor[index(slot)].Move = a.Move
}

// Advance runs one simulation step after this tick's inputs were applied.
// This is the whole of the game's physics, and every line of it is
// integer arithmetic.
func (Simulation) Advance(s *State) {
	if s.Over {
		return
	}
	for i := range s.Actor {
		speed := enemySpeed
		if i == index(Player) {
			speed = playerSpeed
		}
		step(&s.Actor[i], speed)
	}

	// Catching is checked after everyone has moved, so the tick order of
	// the actors cannot decide the outcome.
	player := s.Actor[index(Player)]
	for i := 1; i < Seats; i++ {
		if within(player, s.Actor[i], catchR) {
			s.Caught = true
			s.By = session.SlotID(i + 1)
			break
		}
	}

	s.Tick++
	s.Over = s.Caught || s.Tick >= TargetTicks
}

// step moves one actor by its held direction and clamps it to the field.
func step(a *Actor, speed fixmath.F64) {
	switch a.Move {
	case Up:
		a.Y = a.Y.Sub(speed)
	case Down:
		a.Y = a.Y.Add(speed)
	case Left:
		a.X = a.X.Sub(speed)
	case Right:
		a.X = a.X.Add(speed)
	}
	a.X = a.X.Max(actorR).Min(fieldW.Sub(actorR))
	a.Y = a.Y.Max(actorR).Min(fieldH.Sub(actorR))
}

// within reports whether two actors are closer than r, compared on the
// squared distance so no square root is involved.
func within(a, b Actor, r fixmath.F64) bool {
	dx := a.X.Sub(b.X)
	dy := a.Y.Sub(b.Y)
	return dx.Mul(dx).Add(dy.Mul(dy)) < r.Mul(r)
}

// manhattan is the distance measure the evaluation signal and the sample
// bots use. It avoids a square root, and for "which way should I move"
// it answers the same question a Euclidean distance would.
func manhattan(a, b Actor) fixmath.F64 {
	return a.X.Sub(b.X).Abs().Add(a.Y.Sub(b.Y).Abs())
}

// Project builds a slot's observation.
func (g Simulation) Project(s *State, slot session.SlotID) Observation {
	me := index(slot)
	others := make([]Actor, 0, Seats-1)
	for i, a := range s.Actor {
		if i != me {
			others = append(others, a)
		}
	}
	return Observation{
		You:    slot,
		Self:   s.Actor[me],
		Quarry: s.Actor[index(Player)],
		Others: others,
		Tick:   s.Tick,
		Over:   s.Over,
		Signal: g.Evaluate(s, slot),
	}
}

// Evaluate computes a slot's data:evaluation-signal. The session calls
// this; an agent never scores itself (rule:evaluation-computed-by-session).
//
// The two roles are scored as the opposites they are: the player is
// measured by time survived and distance kept, an enemy by how close it
// has closed. Recording both is what lets a later analysis ask whether an
// enemy kind is actually contributing.
func (Simulation) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	player := s.Actor[index(Player)]
	if slot == Player {
		nearest := maxDist
		for i := 1; i < Seats; i++ {
			nearest = nearest.Min(manhattan(player, s.Actor[i]))
		}
		sig := session.EvaluationSignal{
			Score:      int64(s.Tick),
			Progress:   fixmath.FromInt32(int32(s.Tick)).Div(fixmath.FromInt32(TargetTicks)),
			Evaluation: nearest,
		}
		if s.Over {
			if s.Caught {
				sig.Terminal = session.Lose
			} else {
				sig.Terminal = session.Win
			}
		}
		return sig
	}

	dist := manhattan(s.Actor[index(slot)], player)
	sig := session.EvaluationSignal{
		Score:      int64(dist.FloorToInt()),
		Progress:   fixmath.FromInt32(1).Sub(dist.Div(maxDist)).Max(0),
		Evaluation: dist.Neg(),
	}
	if s.Over {
		if s.Caught {
			sig.Terminal = session.Win
		} else {
			sig.Terminal = session.Lose
		}
	}
	return sig
}

// ErrOver and ErrBadDirection are the rejections Validator can produce.
var (
	ErrOver         = errors.New("the episode is over")
	ErrBadDirection = errors.New("not a direction")
)

// Validator is api:action-validator: what is possible under the rules. It
// runs before Apply and must not mutate the state.
//
// It rejects a direction outside the enum, which is the shape of every
// real legality rule: a client that sends it is broken or lying, and
// either way the rejection is counted and evidenced rather than applied.
type Validator struct{}

// Legal reports whether the action can be applied in this state.
func (Validator) Legal(s *State, slot session.SlotID, a Action) error {
	if s.Over {
		return fmt.Errorf("%w: slot %d", ErrOver, slot)
	}
	if a.Move >= dirCount {
		return fmt.Errorf("%w: %d", ErrBadDirection, a.Move)
	}
	return nil
}

// Tuning is data:session-tuning-profile: the timing and bandwidth
// constants of this game. The framework ships no defaults
// (decision:no-framework-tuning-defaults).
//
// SendRate equals TickRate because the only receiver is a renderer in
// this process; a networked build lowers it and interpolates.
func Tuning() session.TuningProfile {
	return session.TuningProfile{
		TickRate:      60,
		SendRate:      60,
		SnapshotEvery: 120,
		HistoryDepth:  12,
	}
}

// Canonical encodes the committed world into stable bytes — the input of
// data:state-checkpoint. Field order is fixed and every value is an
// integer, so two machines that simulated the same episode produce the
// same digest or the determinism claim is false.
func Canonical(s *State) []byte {
	buf := make([]byte, 0, 8+Seats*17+16)
	buf = binary.BigEndian.AppendUint64(buf, uint64(s.Tick))
	for _, a := range s.Actor {
		buf = binary.BigEndian.AppendUint64(buf, uint64(a.X.Raw()))
		buf = binary.BigEndian.AppendUint64(buf, uint64(a.Y.Raw()))
		buf = append(buf, byte(a.Move))
	}
	caught := byte(0)
	if s.Caught {
		caught = 1
	}
	buf = append(buf, caught, byte(s.By))
	return buf
}

// Config assembles a session over these rules. Every entry point builds
// its session through here, so a rule change reaches all of them.
func Config(id string, seed uint64) session.Config[State, Action, Observation] {
	tuning := Tuning()
	return session.Config[State, Action, Observation]{
		ID:              id,
		Slots:           Slots(),
		Simulation:      Simulation{},
		Validator:       Validator{},
		Seed:            seed,
		Tuning:          &tuning,
		Canonical:       Canonical,
		CheckpointEvery: 60,
	}
}
