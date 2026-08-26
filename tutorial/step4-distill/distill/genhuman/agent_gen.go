// Package genhuman is where the copy of YOUR play goes.
//
// This file is a hand-written placeholder, and the one file here worth
// reading before you overwrite it: until you distill your own recordings
// with
//
//	go run ./cmd/distill --target you
//
// You is silent everywhere — Decide answers nothing — and the seat only
// moves because main.go puts an understudy behind it. Play some games,
// distill, run again: every case that appears in this file afterwards is
// a situation you actually played. `git checkout` on this directory
// brings the empty version back.
package genhuman

import (
	"context"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/game"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/msg"
)

// Decide is the decision list of your play. Empty so far: no recordings
// of yours have been distilled into it.
func Decide(game.Sight) (msg.Move, bool) {
	var zero msg.Move
	return zero, false
}

// You seats the copy behind api:agent-interface, exactly the shape the
// generator writes.
type You struct {
	last game.Sight
	has  bool
}

func (*You) Joined(session.SlotID) {}

func (a *You) Observe(obs game.Sight) { a.last, a.has = obs, true }

func (a *You) Decide(context.Context) (msg.Move, bool) {
	if !a.has {
		var zero msg.Move
		return zero, false
	}
	return Decide(a.last)
}

func (*You) Ended(session.Result) {}
