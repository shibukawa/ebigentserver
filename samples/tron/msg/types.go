// Package msg declares tron's wire types. The trail is an append-only
// identity collection: each tick adds at most one cell per living player,
// so a data:state-delta carries only the new cells while a snapshot
// carries the whole history — exactly the shape that makes deltas clearly
// cheaper than snapshots (sample:tron's synchronization note).
package msg

import (
	"github.com/shibukawa/tinybind-go/cborbind"
)

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

// Grid dimensions; positions are cell coordinates, so plain uint8 needs
// no fixed-point scale.
const (
	GridW = 64
	GridH = 48
)

// TurnInput is data:player-input on the wire profile: an absolute
// direction request.
type TurnInput struct {
	// Tick is the client's tick estimate.
	Tick uint32
	// Dir is 0=up, 1=right, 2=down, 3=left.
	Dir uint8
	// Buttons is reserved.
	Buttons uint8
}

var _ = cborbind.GenerateWireCodec[TurnInput]()

// Player is one light cycle.
type Player struct {
	ID    uint16 `cbor:"id,key=1,identity"`
	X     uint8  `cbor:"x,key=2"`
	Y     uint8  `cbor:"y,key=3"`
	Dir   uint8  `cbor:"dir,key=4"`
	Alive bool   `cbor:"alive,key=5"`
	// DeathTick records when the cycle crashed; survival time is the
	// evaluation signal's score.
	DeathTick uint32 `cbor:"death,key=6"`
}

// TrailCell is one cell of wall left behind a cycle.
type TrailCell struct {
	ID    uint32 `cbor:"id,key=1,identity"`
	X     uint8  `cbor:"x,key=2"`
	Y     uint8  `cbor:"y,key=3"`
	Owner uint16 `cbor:"owner,key=4"`
}

// TronState is the authoritative world on the world profile.
type TronState struct {
	Tick      uint64      `cbor:"tick,key=1"`
	Players   []Player    `cbor:"players,key=2"`
	Trail     []TrailCell `cbor:"trail,key=3"`
	NextTrail uint32      `cbor:"next_trail,key=4"`
	Alive     uint8       `cbor:"alive,key=5"`
	Over      bool        `cbor:"over,key=6"`
	// Winner is the surviving slot, 0 on a mutual crash or timeout.
	Winner uint16 `cbor:"winner,key=7"`
}

var _ = cborbind.GenerateWorldDelta[TronState]()
