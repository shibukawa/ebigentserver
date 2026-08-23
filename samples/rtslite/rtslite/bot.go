package rtslite

import (
	"context"

	"github.com/shibukawa/ebigentserver/samples/rtslite/msg"
	"github.com/shibukawa/ebigentserver/session"
)

// Bot commands its army from the fog view: units with a visible enemy
// charge the nearest one; the rest sweep toward the map center. It
// issues several orders per decision — the command-stream shape
// IntakeAll exists for. Minimal on purpose.
type Bot struct {
	last Sight
	// OrdersPerDecide bounds the burst; 0 means 4.
	OrdersPerDecide int
	queue           []Input
}

var _ session.Agent[Sight, Input] = (*Bot)(nil)

func (*Bot) Joined(session.SlotID) {}
func (b *Bot) Observe(obs Sight)   { b.last = obs; b.plan() }
func (*Bot) Ended(session.Result)  {}

// Decide pops one planned order per call; the client loop sends one per
// received view, so plans drain over a few ticks.
func (b *Bot) Decide(context.Context) (Input, bool) {
	if len(b.queue) == 0 {
		return Input{}, false
	}
	in := b.queue[0]
	b.queue = b.queue[1:]
	return in, true
}

func (b *Bot) plan() {
	v := b.last.View
	if v == nil || v.Over {
		b.queue = nil
		return
	}
	limit := b.OrdersPerDecide
	if limit == 0 {
		limit = 4
	}
	b.queue = b.queue[:0]
	for i, u := range v.Own {
		if len(b.queue) >= limit {
			break
		}
		// Re-order a unit only when idle or every few ticks, so the
		// stream stays a stream rather than a flood.
		if uint64(i)%4 != v.Tick/8%4 {
			continue
		}
		tx, ty := uint8(msg.MapW/2), uint8(msg.MapH/2)
		best := int16(0x7FFF)
		for _, e := range v.Visible {
			if d := chebyshev(u.X, u.Y, e.X, e.Y); d < best {
				best, tx, ty = d, e.X, e.Y
			}
		}
		if u.TX != tx || u.TY != ty {
			b.queue = append(b.queue, Input{Tick: uint32(v.Tick), Unit: u.ID, TargetX: tx, TargetY: ty})
		}
	}
}
