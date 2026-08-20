// Package msg declares the dungeon sample's wire types. This sample's
// point is that the two receiver views differ in kind, not radius: the
// dungeon master's view and an adventurer's view are different structs,
// and the full DungeonState is never a wire message at all — the
// projection runs before serialization
// (policy:observation-scoped-information).
package msg

import (
	"github.com/shibukawa/tinybind-go/cborbind"
)

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

// Grid dimensions; the explored and wall bitmaps are row-major, one bit
// per cell.
const (
	GridW = 32
	GridH = 24
	// BitmapLen is the byte length of one grid bitmap.
	BitmapLen = GridW * GridH / 8
)

// Unknown marks a coordinate the receiver is not allowed to know.
const Unknown = 0xFF

// Roles, assigned to adventurer slots in admission order.
const (
	RoleDM uint8 = iota
	RoleScout
	RoleEngineer
	RoleCarrier
	RoleNavigator
)

// Action kinds.
const (
	ActMove uint8 = iota
	ActPlaceTrap
	ActDisarm
)

// ActionInput is data:player-input for every slot: adventurers move and
// disarm, the dungeon master places traps.
type ActionInput struct {
	Tick uint32
	Kind uint8
	// Dir is 0=up 1=right 2=down 3=left for ActMove.
	Dir uint8
	// X, Y target ActPlaceTrap and ActDisarm.
	X uint8
	Y uint8
}

var _ = cborbind.GenerateWireCodec[ActionInput]()

// Adventurer is one party member in the full state and the DM view.
type Adventurer struct {
	ID       uint16 `cbor:"id,key=1,identity"`
	X        uint8  `cbor:"x,key=2"`
	Y        uint8  `cbor:"y,key=3"`
	Role     uint8  `cbor:"role,key=4"`
	HP       int8   `cbor:"hp,key=5"`
	Alive    bool   `cbor:"alive,key=6"`
	Carrying bool   `cbor:"carrying,key=7"`
}

// Trap is a dungeon master device. Discovered means the party has seen
// it; only discovered traps ever reach an adventurer view.
type Trap struct {
	ID         uint32 `cbor:"id,key=1,identity"`
	X          uint8  `cbor:"x,key=2"`
	Y          uint8  `cbor:"y,key=3"`
	Armed      bool   `cbor:"armed,key=4"`
	Discovered bool   `cbor:"disc,key=5"`
}

// DungeonState is the authoritative world (concept:world-state). It is
// used for simulation, canonical checkpoints, and recording — never as a
// wire message to a client.
type DungeonState struct {
	Tick        uint64       `cbor:"tick,key=1"`
	Walls       []uint8      `cbor:"walls,key=2"`
	Explored    []uint8      `cbor:"explored,key=3"`
	Adventurers []Adventurer `cbor:"advs,key=4"`
	Traps       []Trap       `cbor:"traps,key=5"`
	TrapBudget  uint8        `cbor:"budget,key=6"`
	TreasureX   uint8        `cbor:"tx,key=7"`
	TreasureY   uint8        `cbor:"ty,key=8"`
	ExitX       uint8        `cbor:"ex,key=9"`
	ExitY       uint8        `cbor:"ey,key=10"`
	TickLimit   uint32       `cbor:"limit,key=11"`
	Over        bool         `cbor:"over,key=12"`
	// Winner: 0 none, 1 party, 2 dungeon master.
	Winner uint8 `cbor:"winner,key=13"`
}

var _ = cborbind.GenerateWorldDelta[DungeonState]()

// PartyMate is a teammate as an adventurer sees one (team scope).
type PartyMate struct {
	ID       uint16 `cbor:"id,key=1,identity"`
	X        uint8  `cbor:"x,key=2"`
	Y        uint8  `cbor:"y,key=3"`
	Role     uint8  `cbor:"role,key=4"`
	Alive    bool   `cbor:"alive,key=5"`
	Carrying bool   `cbor:"carrying,key=6"`
}

// AdventurerView is the party wire view: self and team scopes plus role
// extras. Walls appear only inside explored cells; traps only once
// discovered; the exit only for the navigator (concept:visibility-scope).
type AdventurerView struct {
	Tick     uint64 `cbor:"tick,key=1"`
	You      uint16 `cbor:"you,key=2"`
	Role     uint8  `cbor:"role,key=3"`
	HP       int8   `cbor:"hp,key=4"`
	X        uint8  `cbor:"x,key=5"`
	Y        uint8  `cbor:"y,key=6"`
	Carrying bool   `cbor:"carrying,key=7"`
	// Explored is the team's accumulated knowledge (team scope).
	Explored []uint8 `cbor:"explored,key=8"`
	// KnownWalls is the wall bitmap masked to explored cells.
	KnownWalls []uint8 `cbor:"walls,key=9"`
	// Party is every teammate's position (team scope).
	Party []PartyMate `cbor:"party,key=10"`
	// KnownTraps holds only discovered traps.
	KnownTraps []Trap `cbor:"traps,key=11"`
	// TreasureX/Y are Unknown until the treasure's cell is explored.
	TreasureX uint8 `cbor:"tx,key=12"`
	TreasureY uint8 `cbor:"ty,key=13"`
	// ExitX/Y are Unknown for everyone but the navigator (role scope).
	ExitX  uint8 `cbor:"ex,key=14"`
	ExitY  uint8 `cbor:"ey,key=15"`
	Over   bool  `cbor:"over,key=16"`
	Winner uint8 `cbor:"winner,key=17"`
}

var _ = cborbind.GenerateWorldDelta[AdventurerView]()

// DMView is the dungeon master wire view: the whole map (role scope),
// including what the party has explored so far.
type DMView struct {
	Tick        uint64       `cbor:"tick,key=1"`
	Walls       []uint8      `cbor:"walls,key=2"`
	Explored    []uint8      `cbor:"explored,key=3"`
	Adventurers []Adventurer `cbor:"advs,key=4"`
	Traps       []Trap       `cbor:"traps,key=5"`
	TrapBudget  uint8        `cbor:"budget,key=6"`
	TreasureX   uint8        `cbor:"tx,key=7"`
	TreasureY   uint8        `cbor:"ty,key=8"`
	ExitX       uint8        `cbor:"ex,key=9"`
	ExitY       uint8        `cbor:"ey,key=10"`
	Over        bool         `cbor:"over,key=11"`
	Winner      uint8        `cbor:"winner,key=12"`
}

var _ = cborbind.GenerateWorldDelta[DMView]()
