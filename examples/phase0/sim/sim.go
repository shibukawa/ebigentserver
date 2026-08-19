// Package sim exercises the whole Phase 0 stack in one deterministic
// episode: fixed-point math (api:fixed-point-math via fixmath), declared-scale
// wire quantization, generated CBOR codecs, and framework-side deltas. The
// digest it produces must be identical on every architecture — the property
// Phase 2 will assert across arm64 and amd64.
package sim

import (
	"fmt"
	"hash/fnv"

	"github.com/shibukawa/ebigentserver/examples/phase0/msg"
	"github.com/shibukawa/fixmath"
)

// Run advances an orbiting two-entity world for the given number of ticks,
// encoding a delta per tick, and returns an FNV-1a digest over every encoded
// delta. Same ticks, same digest — on every target.
func Run(ticks int) uint64 {
	world := msg.WorldState{
		Entities: []msg.Entity{
			{ID: 1, HP: 100},
			{ID: 2, HP: 100},
		},
		Phase: 1,
	}

	// Orbit parameters in compute format: radius 5.25 units, one revolution
	// per 256 ticks for entity 1, per 384 ticks for entity 2.
	radius := fixmath.FromScaled(5*1024+256, 10) // 5.25 exactly, from the wire scale
	turn1 := fixmath.Angle(1 << 24)              // 2^32 / 256 ticks
	turn2 := fixmath.Angle(11184811)             // ~2^32 / 384, wrapping is free

	digest := fnv.New64a()
	buf := make([]byte, 0, 256)

	// The retained baseline must be an independent copy: WorldState holds a
	// slice, and a shallow copy would alias the entities, making every diff
	// empty. Framework-side snapshot retention (Phase 3a,
	// decision:framework-side-delta-generation) owns this concern for real
	// games; here the copy is explicit.
	baseline := world
	baseline.Entities = append([]msg.Entity(nil), world.Entities...)

	var a1, a2 fixmath.Angle
	for range ticks {
		world.Tick++
		a1 += turn1
		a2 += turn2

		p1 := fixmath.Vec2FromAngle(a1, radius)
		p2 := fixmath.Vec2FromAngle(a2, radius)

		// Quantize compute values onto the declared wire scales; speed is the
		// chord length between consecutive positions, exercising Sqrt.
		e1, e2 := &world.Entities[0], &world.Entities[1]
		prev1 := fixmath.Vec2{X: e1.PosX.F64(), Y: e1.PosY.F64()}
		e1.Vel = msg.Fixed65536FromF64(p1.Distance(prev1))
		e1.PosX, e1.PosY = msg.Fixed1024FromF64(p1.X), msg.Fixed1024FromF64(p1.Y)

		heading := p2.Sub(fixmath.Vec2{X: e2.PosX.F64(), Y: e2.PosY.F64()}).Angle()
		_ = heading // Atan2 participates in the episode even though only position is carried
		e2.Vel = msg.Fixed65536FromF64(p2.Length())
		e2.PosX, e2.PosY = msg.Fixed1024FromF64(p2.X), msg.Fixed1024FromF64(p2.Y)

		delta := msg.DiffWorldState(baseline, world)
		buf = delta.AppendCBORTo(buf[:0])
		if _, err := digest.Write(buf); err != nil {
			panic(err)
		}
		if err := msg.ApplyWorldStateDelta(&baseline, delta); err != nil {
			panic(fmt.Sprintf("tick %d: apply: %v", world.Tick, err))
		}
	}
	return digest.Sum64()
}
