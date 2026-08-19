// The dedicated-server entry point (concept:dedicated-server-mode). It never
// imports the engine: per decision:entry-points-over-build-tags the Go linker
// excludes what an entry point does not import, with no build tag anywhere.
package main

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/examples/phase0/msg"
)

func main() {
	fmt.Println("phase0 dedicated server (headless)")
	fmt.Println("protocol version:", msg.CBORProtocolVersion)

	in := msg.PlayerInput{Tick: 1, MoveX: 1024, MoveY: -512, Buttons: 1}
	encoded := in.AppendCBORTo(nil)
	fmt.Printf("PlayerInput wire encoding: %x (%d bytes)\n", encoded, len(encoded))
}
