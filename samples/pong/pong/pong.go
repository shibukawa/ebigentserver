// Package pong is sample:pong — the minimal realtime game: continuous
// input, a fixed server tick, authoritative fixed-point physics, and
// snapshot/delta downstream. Everything before this sample was request
// and response; this one exercises the tick loop.
package pong

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/samples/pong/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/fixmath"
)

// The two player slots: left defends x=0, right defends x=Width.
const (
	SlotLeft  session.SlotID = 1
	SlotRight session.SlotID = 2
)

// Slots is the game-defined slot set.
func Slots() []session.SlotID { return []session.SlotID{SlotLeft, SlotRight} }

// WinScore ends the game.
const WinScore = 5

// Field geometry and speeds, in compute units (1 unit = 1/1024 on the
// wire). All constants are exact in fixed point.
var (
	width      = fixmath.FromInt32(320)
	height     = fixmath.FromInt32(180)
	paddleX    = fixmath.FromInt32(8)                           // paddle plane distance from each edge
	paddleHalf = fixmath.FromInt32(16)                          // half paddle height
	paddleStep = fixmath.FromInt32(3)                           // per-tick paddle movement
	ballSpeedX = fixmath.FromInt32(5).Div(fixmath.FromInt32(2)) // 2.5/tick
	spinFactor = fixmath.FromInt32(1).Div(fixmath.FromInt32(8)) // vy gain per unit of hit offset
	serveVelY  = fixmath.FromInt32(1)
)

// State aliases the generated wire state: the authoritative world IS the
// wire shape (decision:go-struct-world-state), so snapshots, deltas, and
// checkpoints all read one struct.
type State = msg.PongState

// Input aliases the wire input message.
type Input = msg.PaddleInput

// Observation is the game's concept:observation: global scope, the whole
// state plus the slot's evaluation signal.
type Observation struct {
	You    session.SlotID
	State  State
	Signal session.EvaluationSignal
}

// RuleSet implements session.TickStageRuleSet. The zero value is ready to use.
type RuleSet struct{}

var _ session.TickStageRuleSet[State, Input, Observation] = RuleSet{}

// Start serves the first ball to the right; the shared seed is unused —
// pong's serve pattern is score-driven, not random.
func (RuleSet) Start(uint64) State {
	s := State{}
	center(&s, true)
	s.LeftY = msg.Fixed1024FromF64(height.Div(fixmath.FromInt32(2)))
	s.RightY = s.LeftY
	return s
}

// ActingSlots is unused in realtime pacing; it reports the open slots so
// the step-paced loop would not spin.
func (RuleSet) ActingSlots(s *State) []session.SlotID {
	if s.Over {
		return nil
	}
	return Slots()
}

// Apply moves the slot's paddle by one validated input.
func (RuleSet) Apply(s *State, slot session.SlotID, in Input) {
	step := paddleStep.Mul(fixmath.FromInt32(int32(in.MoveY)))
	lo, hi := paddleHalf, height.Sub(paddleHalf)
	if slot == SlotLeft {
		s.LeftY = msg.Fixed1024FromF64(clamp(s.LeftY.F64().Add(step), lo, hi))
	} else {
		s.RightY = msg.Fixed1024FromF64(clamp(s.RightY.F64().Add(step), lo, hi))
	}
}

// Advance runs one tick of authoritative physics in fixed point.
func (RuleSet) Advance(s *State) {
	if s.Over {
		return
	}
	bx := s.BallX.F64().Add(s.VelX.F64())
	by := s.BallY.F64().Add(s.VelY.F64())
	vx, vy := s.VelX.F64(), s.VelY.F64()

	// Wall bounces.
	if by < fixmath.Zero {
		by, vy = by.Neg(), vy.Neg()
	}
	if by > height {
		by, vy = height.Add(height).Sub(by), vy.Neg()
	}

	// Paddle planes. A ball crossing the plane toward the edge either
	// reflects off the paddle (gaining spin from the hit offset) or
	// scores.
	leftPlane, rightPlane := paddleX, width.Sub(paddleX)
	if vx < fixmath.Zero && bx <= leftPlane {
		if hit, off := paddleHit(by, s.LeftY.F64()); hit {
			bx = leftPlane.Add(leftPlane).Sub(bx)
			vx = vx.Neg()
			vy = vy.Add(off.Mul(spinFactor))
		} else if bx < fixmath.Zero {
			s.ScoreR++
			score(s, false)
			return
		}
	}
	if vx > fixmath.Zero && bx >= rightPlane {
		if hit, off := paddleHit(by, s.RightY.F64()); hit {
			bx = rightPlane.Add(rightPlane).Sub(bx)
			vx = vx.Neg()
			vy = vy.Add(off.Mul(spinFactor))
		} else if bx > width {
			s.ScoreL++
			score(s, true)
			return
		}
	}

	s.BallX = msg.Fixed1024FromF64(bx)
	s.BallY = msg.Fixed1024FromF64(by)
	s.VelX = msg.Fixed65536FromF64(vx)
	s.VelY = msg.Fixed65536FromF64(vy)
	s.Tick++
}

// Project builds a slot's observation (global visibility).
func (g RuleSet) Project(s *State, slot session.SlotID) Observation {
	return Observation{You: slot, State: *s, Signal: g.Evaluate(s, slot)}
}

// Evaluate computes the slot's data:evaluation-signal.
func (RuleSet) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	own, opp := int64(s.ScoreL), int64(s.ScoreR)
	if slot == SlotRight {
		own, opp = opp, own
	}
	lead := fixmath.FromInt32(int32(maxI(int64(s.ScoreL), int64(s.ScoreR))))
	sig := session.EvaluationSignal{
		Score:      own,
		Evaluation: fixmath.FromInt32(int32(own - opp)),
		Progress:   lead.Div(fixmath.FromInt32(WinScore)),
	}
	if !s.Over {
		return sig
	}
	switch {
	case uint16(slot) == s.Winner:
		sig.Terminal = session.Win
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

// Validator is the legality class of api:action-validator.
type Validator struct{}

// Legal accepts MoveY in {-1, 0, 1}.
func (Validator) Legal(s *State, slot session.SlotID, in Input) error {
	if in.MoveY < -1 || in.MoveY > 1 {
		return fmt.Errorf("pong: MoveY %d out of range", in.MoveY)
	}
	return nil
}

// Codec wires the generated snapshot and delta functions into statesync
// (decision:framework-side-delta-generation). PongState has no reference
// fields, so the default value copy is a correct Clone.
func Codec() statesync.Codec[State, msg.PongStateDelta] {
	return statesync.Codec[State, msg.PongStateDelta]{
		AppendSnapshot: func(dst []byte, s *State) []byte { return s.AppendCBORTo(dst) },
		DecodeSnapshot: func(s *State, data []byte) error { return s.DecodeCBORFrom(data) },
		Diff: func(base, cur *State) msg.PongStateDelta {
			return msg.DiffPongState(*base, *cur)
		},
		AppendDelta: func(dst []byte, d *msg.PongStateDelta) []byte { return d.AppendCBORTo(dst) },
		DecodeDelta: func(d *msg.PongStateDelta, data []byte) error { return d.DecodeCBORFrom(data) },
		ApplyDelta:  msg.ApplyPongStateDelta,
	}
}

// Canonical encodes the state for data:state-checkpoint — the CBOR world
// profile encoding is the canonical input, exactly as the concept asks.
func Canonical(s *State) []byte { return s.AppendCBORTo(nil) }

// center places the ball mid-field serving toward the given side.
func center(s *State, toRight bool) {
	s.BallX = msg.Fixed1024FromF64(width.Div(fixmath.FromInt32(2)))
	s.BallY = msg.Fixed1024FromF64(height.Div(fixmath.FromInt32(2)))
	vx := ballSpeedX
	if !toRight {
		vx = vx.Neg()
	}
	// Serve angle alternates with the rally count so play is varied but
	// fully deterministic.
	vy := serveVelY
	if (s.ScoreL+s.ScoreR)%2 == 1 {
		vy = vy.Neg()
	}
	s.VelX = msg.Fixed65536FromF64(vx)
	s.VelY = msg.Fixed65536FromF64(vy)
}

// score books a point, ends or re-serves, and advances the tick.
func score(s *State, leftScored bool) {
	if s.ScoreL >= WinScore || s.ScoreR >= WinScore {
		s.Over = true
		if s.ScoreL > s.ScoreR {
			s.Winner = uint16(SlotLeft)
		} else {
			s.Winner = uint16(SlotRight)
		}
		s.VelX, s.VelY = 0, 0
		s.Tick++
		return
	}
	// Loser receives the serve.
	center(s, !leftScored)
	s.Tick++
}

func paddleHit(ballY, paddleY fixmath.F64) (bool, fixmath.F64) {
	off := ballY.Sub(paddleY)
	if off.Abs() > paddleHalf {
		return false, fixmath.Zero
	}
	return true, off
}

func clamp(v, lo, hi fixmath.F64) fixmath.F64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxI(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
