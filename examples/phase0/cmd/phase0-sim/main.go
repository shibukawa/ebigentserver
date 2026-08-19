// The simulation entry point (concept:simulation-mode): session and local
// state only, no transport, no engine. It advances a fixed-point world for a
// fixed number of ticks and prints a digest of every encoded state — the
// seed of Phase 2's cross-architecture determinism check.
package main

import (
	"fmt"
	"hash/fnv"

	"github.com/shibukawa/ebigentserver/examples/phase0/msg"
)

func main() {
	world := msg.WorldState{
		Tick: 0,
		Entities: []msg.Entity{
			{ID: 1, PosX: 0, PosY: 0, Vel: 1 << 16, HP: 100},
			{ID: 2, PosX: 10240, PosY: -10240, Vel: 3 << 14, HP: 100},
		},
		Phase: 1,
	}

	digest := fnv.New64a()
	buf := make([]byte, 0, 256)
	baseline := world

	const ticks = 1000
	for range ticks {
		world.Tick++
		for i := range world.Entities {
			e := &world.Entities[i]
			// Velocity is at 1/65536, position at 1/1024: shift 6 converts.
			e.PosX += msg.Fixed1024(int64(e.Vel) >> 6)
			e.PosY -= msg.Fixed1024(int64(e.Vel) >> 7)
		}

		delta := msg.DiffWorldState(baseline, world)
		buf = delta.AppendCBORTo(buf[:0])
		if _, err := digest.Write(buf); err != nil {
			panic(err)
		}
		if err := msg.ApplyWorldStateDelta(&baseline, delta); err != nil {
			panic(fmt.Sprintf("tick %d: apply: %v", world.Tick, err))
		}
	}

	fmt.Println("phase0 simulation (headless, deterministic)")
	fmt.Println("protocol version:", msg.CBORProtocolVersion)
	fmt.Printf("ticks: %d, episode digest: %016x\n", ticks, digest.Sum64())
}
