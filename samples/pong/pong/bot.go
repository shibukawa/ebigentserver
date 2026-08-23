package pong

import (
	"context"

	"github.com/shibukawa/ebigentserver/samples/pong/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/fixmath"
)

// Bot chases the ball: move toward it when the offset exceeds a dead
// zone. Deliberately minimal (the project's AI depth comes from episode
// distillation, not hand-written play).
type Bot struct {
	last Observation
}

var _ session.Agent[Observation, Input] = (*Bot)(nil)

var deadZone = fixmath.FromInt32(4)

// Guest does nothing.
func (*Bot) Guest(session.SlotID) {}

// Observe retains the latest observation.
func (b *Bot) Observe(obs Observation) { b.last = obs }

// Decide moves toward the ball's height.
func (b *Bot) Decide(context.Context) (Input, bool) {
	s := &b.last.State
	my := s.LeftY
	if b.last.You == SlotRight {
		my = s.RightY
	}
	off := s.BallY.F64().Sub(my.F64())
	in := Input{Tick: uint32(s.Tick)}
	switch {
	case off > deadZone:
		in.MoveY = 1
	case off < deadZone.Neg():
		in.MoveY = -1
	default:
		return in, false // no input this tick
	}
	return in, true
}

// Ended does nothing.
func (*Bot) Ended(session.Result) {}

var _ = msg.PaddleInput{} // keep the wire package visibly the input source
