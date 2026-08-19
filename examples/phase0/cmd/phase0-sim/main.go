// The simulation entry point (concept:simulation-mode): session and local
// state only, no transport, no engine. It advances a fixed-point world —
// fixmath trigonometry, declared-scale quantization, generated CBOR deltas —
// and prints a digest of every encoded delta: the seed of Phase 2's
// cross-architecture determinism check.
package main

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/examples/phase0/msg"
	"github.com/shibukawa/ebigentserver/examples/phase0/sim"
)

func main() {
	const ticks = 1000
	fmt.Println("phase0 simulation (headless, deterministic)")
	fmt.Println("protocol version:", msg.CBORProtocolVersion)
	fmt.Printf("ticks: %d, episode digest: %016x\n", ticks, sim.Run(ticks))
}
