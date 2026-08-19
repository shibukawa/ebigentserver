// The client entry point: the one build target allowed to import
// system:ebitengine (rule:engine-import-confined-to-client-entry). Phase 0
// has nothing to render, so the engine import arrives with Phase 1 — but the
// entry point exists now because the cmd layout is the seam
// decision:entry-points-over-build-tags reserves.
package main

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/examples/phase0/msg"
)

func main() {
	fmt.Println("phase0 client (rendering arrives in Phase 1)")
	fmt.Println("protocol version:", msg.CBORProtocolVersion)
}
