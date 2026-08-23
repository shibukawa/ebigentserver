package dungeon

import (
	"context"

	"github.com/shibukawa/ebigentserver/samples/dungeon/msg"
	"github.com/shibukawa/ebigentserver/session"
)

// DMBot places a trap ahead of the party every interval — minimal, like
// every sample bot; it decides purely from its DMView.
type DMBot struct {
	last     Sight
	lastPlay uint64
}

var _ session.Agent[Sight, Input] = (*DMBot)(nil)

func (*DMBot) Joined(session.SlotID) {}
func (b *DMBot) Observe(obs Sight)   { b.last = obs }
func (*DMBot) Ended(session.Result)  {}

// Decide drops a trap two cells ahead of the first living adventurer.
func (b *DMBot) Decide(context.Context) (Input, bool) {
	v := b.last.DM
	if v == nil || v.Over || v.TrapBudget == 0 || v.Tick < b.lastPlay+20 {
		return Input{}, false
	}
	for _, a := range v.Adventurers {
		if !a.Alive {
			continue
		}
		for _, c := range [][2]int16{{2, 0}, {0, 2}, {-2, 0}, {0, -2}, {1, 1}} {
			x, y := int16(a.X)+c[0], int16(a.Y)+c[1]
			if x < 1 || y < 1 || x >= msg.GridW-1 || y >= msg.GridH-1 {
				continue
			}
			ux, uy := uint8(x), uint8(y)
			if Bit(v.Walls, ux, uy) || (ux == v.ExitX && uy == v.ExitY) ||
				(ux == v.TreasureX && uy == v.TreasureY) || occupied(v, ux, uy) {
				continue
			}
			b.lastPlay = v.Tick
			return Input{Tick: uint32(v.Tick), Kind: msg.ActPlaceTrap, X: ux, Y: uy}, true
		}
	}
	return Input{}, false
}

func occupied(v *msg.DMView, x, y uint8) bool {
	for _, a := range v.Adventurers {
		if a.Alive && a.X == x && a.Y == y {
			return true
		}
	}
	return false
}

// AdventurerBot walks toward what its view lets it know: a navigator
// heads for the exit, a carrier for a seen treasure, everyone else drifts
// along the explored frontier. It reads only the AdventurerView.
type AdventurerBot struct {
	last  Sight
	prevX uint8
	prevY uint8
}

var _ session.Agent[Sight, Input] = (*AdventurerBot)(nil)

func (*AdventurerBot) Joined(session.SlotID) {}
func (b *AdventurerBot) Observe(obs Sight)   { b.last = obs }
func (*AdventurerBot) Ended(session.Result)  {}

func (b *AdventurerBot) Decide(context.Context) (Input, bool) {
	v := b.last.Adventurer
	if v == nil || v.Over || v.HP <= 0 {
		return Input{}, false
	}
	tx, ty, ok := b.target(v)
	dirs := preferDirs(v.X, v.Y, tx, ty, ok, v.Tick)
	for _, d := range dirs {
		nx, ny, valid := step(v.X, v.Y, d)
		if !valid || Bit(v.KnownWalls, nx, ny) {
			continue
		}
		if nx == b.prevX && ny == b.prevY {
			continue // no immediate backtracking
		}
		if trapAt(v, nx, ny) {
			continue // never walk onto a discovered armed trap
		}
		b.prevX, b.prevY = v.X, v.Y
		return Input{Tick: uint32(v.Tick), Kind: msg.ActMove, Dir: d}, true
	}
	b.prevX, b.prevY = msg.Unknown, msg.Unknown // stuck: allow backtrack next tick
	return Input{}, false
}

func (b *AdventurerBot) target(v *msg.AdventurerView) (uint8, uint8, bool) {
	if v.Carrying && v.ExitX != msg.Unknown {
		return v.ExitX, v.ExitY, true
	}
	if v.Role == msg.RoleCarrier && v.TreasureX != msg.Unknown {
		return v.TreasureX, v.TreasureY, true
	}
	if v.Role == msg.RoleNavigator {
		return v.ExitX, v.ExitY, true
	}
	return 0, 0, false
}

// preferDirs orders directions toward the target, or sweeps by tick
// parity when no target is known.
func preferDirs(x, y, tx, ty uint8, has bool, tick uint64) []uint8 {
	if !has {
		if tick/32%2 == 0 {
			return []uint8{1, 2, 0, 3}
		}
		return []uint8{2, 1, 3, 0}
	}
	var first, second uint8
	if tx > x {
		first = 1
	} else {
		first = 3
	}
	if ty > y {
		second = 2
	} else {
		second = 0
	}
	dx, dy := int16(tx)-int16(x), int16(ty)-int16(y)
	if dy < 0 {
		dy = -dy
	}
	if dx < 0 {
		dx = -dx
	}
	if dy > dx {
		first, second = second, first
	}
	return []uint8{first, second, (first + 2) % 4, (second + 2) % 4}
}

func trapAt(v *msg.AdventurerView, x, y uint8) bool {
	for _, tr := range v.KnownTraps {
		if tr.Armed && tr.X == x && tr.Y == y {
			return true
		}
	}
	return false
}
