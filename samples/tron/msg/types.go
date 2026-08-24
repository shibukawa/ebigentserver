// Package msg declares tron's wire types. The trail is an append-only
// identity collection: each tick adds at most one cell per living player,
// so a data:state-delta carries only the new cells while a snapshot
// carries the whole history — exactly the shape that makes deltas clearly
// cheaper than snapshots (sample:tron's synchronization note).
package msg

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

// Player is one light cycle.
type Player struct {
	ID    uint16 `json:"id"`
	X     uint8  `json:"x"`
	Y     uint8  `json:"y"`
	Dir   uint8  `json:"dir"`
	Alive bool   `json:"alive"`
	// DeathTick records when the cycle crashed; survival time is the
	// evaluation signal's score.
	DeathTick uint32 `json:"death"`
}

// TrailCell is one cell of wall left behind a cycle.
type TrailCell struct {
	ID    uint32 `json:"id"`
	X     uint8  `json:"x"`
	Y     uint8  `json:"y"`
	Owner uint16 `json:"owner"`
}

// TronState is the authoritative world on the world profile.
type TronState struct {
	Tick      uint64      `json:"tick"`
	Players   []Player    `json:"players"`
	Trail     []TrailCell `json:"trail"`
	NextTrail uint32      `json:"next_trail"`
	Alive     uint8       `json:"alive"`
	Over      bool        `json:"over"`
	// Winner is the surviving slot, 0 on a mutual crash or timeout.
	Winner uint16 `json:"winner"`
}
