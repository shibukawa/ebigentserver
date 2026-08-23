// Package msg declares the dungeon sample's wire types. This sample's
// point is that the two receiver views differ in kind, not radius: the
// dungeon master's view and an adventurer's view are different structs,
// and the full DungeonState is never a wire message at all — the
// projection runs before serialization
// (policy:sight-scoped-information).
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

// Adventurer is one party member in the full state and the DM view.
type Adventurer struct {
	ID       uint16 `json:"id"`
	X        uint8  `json:"x"`
	Y        uint8  `json:"y"`
	Role     uint8  `json:"role"`
	HP       int8   `json:"hp"`
	Alive    bool   `json:"alive"`
	Carrying bool   `json:"carrying"`
}

// Trap is a dungeon master device. Discovered means the party has seen
// it; only discovered traps ever reach an adventurer view.
type Trap struct {
	ID         uint32 `json:"id"`
	X          uint8  `json:"x"`
	Y          uint8  `json:"y"`
	Armed      bool   `json:"armed"`
	Discovered bool   `json:"disc"`
}

// DungeonState is the authoritative world (concept:world-state). It is
// used for simulation, canonical checkpoints, and recording — never as a
// wire message to a client.
type DungeonState struct {
	Tick        uint64       `json:"tick"`
	Walls       []uint8      `json:"walls"`
	Explored    []uint8      `json:"explored"`
	Adventurers []Adventurer `json:"advs"`
	Traps       []Trap       `json:"traps"`
	TrapBudget  uint8        `json:"budget"`
	TreasureX   uint8        `json:"tx"`
	TreasureY   uint8        `json:"ty"`
	ExitX       uint8        `json:"ex"`
	ExitY       uint8        `json:"ey"`
	TickLimit   uint32       `json:"limit"`
	Over        bool         `json:"over"`
	// Winner: 0 none, 1 party, 2 dungeon master.
	Winner uint8 `json:"winner"`
}

// PartyMate is a teammate as an adventurer sees one (team scope).
type PartyMate struct {
	ID       uint16 `json:"id"`
	X        uint8  `json:"x"`
	Y        uint8  `json:"y"`
	Role     uint8  `json:"role"`
	Alive    bool   `json:"alive"`
	Carrying bool   `json:"carrying"`
}

// AdventurerView is the party wire view: self and team scopes plus role
// extras. Walls appear only inside explored cells; traps only once
// discovered; the exit only for the navigator (concept:visibility-scope).
type AdventurerView struct {
	Tick     uint64 `json:"tick"`
	You      uint16 `json:"you"`
	Role     uint8  `json:"role"`
	HP       int8   `json:"hp"`
	X        uint8  `json:"x"`
	Y        uint8  `json:"y"`
	Carrying bool   `json:"carrying"`
	// Explored is the team's accumulated knowledge (team scope).
	Explored []uint8 `json:"explored"`
	// KnownWalls is the wall bitmap masked to explored cells.
	KnownWalls []uint8 `json:"walls"`
	// Party is every teammate's position (team scope).
	Party []PartyMate `json:"party"`
	// KnownTraps holds only discovered traps.
	KnownTraps []Trap `json:"traps"`
	// TreasureX/Y are Unknown until the treasure's cell is explored.
	TreasureX uint8 `json:"tx"`
	TreasureY uint8 `json:"ty"`
	// ExitX/Y are Unknown for everyone but the navigator (role scope).
	ExitX  uint8 `json:"ex"`
	ExitY  uint8 `json:"ey"`
	Over   bool  `json:"over"`
	Winner uint8 `json:"winner"`
}

// DMView is the dungeon master wire view: the whole map (role scope),
// including what the party has explored so far.
type DMView struct {
	Tick        uint64       `json:"tick"`
	Walls       []uint8      `json:"walls"`
	Explored    []uint8      `json:"explored"`
	Adventurers []Adventurer `json:"advs"`
	Traps       []Trap       `json:"traps"`
	TrapBudget  uint8        `json:"budget"`
	TreasureX   uint8        `json:"tx"`
	TreasureY   uint8        `json:"ty"`
	ExitX       uint8        `json:"ex"`
	ExitY       uint8        `json:"ey"`
	Over        bool         `json:"over"`
	Winner      uint8        `json:"winner"`
}

// The calls below are what ask the generator for each codec: there is no
// declaration to write any more, and naming an entry point is the ask
// (requirement:cborbind-migration).
//
// Which container a type uses is a contract rather than a preference. An
// input is an array — positional, no field names on the wire, and both
// ends rebuilt together — which is concept:cbor-wire-profile. A world
// state is a map, so a decoder can skip a key it does not know and the two
// ends may ship apart, which is concept:cbor-world-profile.

// AppendActionInput writes one actioninput in the array shape.
func AppendActionInput(dst []byte, v ActionInput) []byte { return cborbind.AppendCBORInArrayTo(dst, v) }

// DecodeActionInput reads one actioninput.
func DecodeActionInput(data []byte) (ActionInput, error) {
	return cborbind.DecodeCBORInArrayFrom[ActionInput](data)
}

// AppendDungeonState writes one dungeonstate in the map shape.
func AppendDungeonState(dst []byte, v DungeonState) []byte { return cborbind.AppendCBORInMapTo(dst, v) }

// DecodeDungeonState reads one dungeonstate.
func DecodeDungeonState(data []byte) (DungeonState, error) {
	return cborbind.DecodeCBORInMapFrom[DungeonState](data)
}

// AppendAdventurerView writes one adventurerview in the map shape.
func AppendAdventurerView(dst []byte, v AdventurerView) []byte {
	return cborbind.AppendCBORInMapTo(dst, v)
}

// DecodeAdventurerView reads one adventurerview.
func DecodeAdventurerView(data []byte) (AdventurerView, error) {
	return cborbind.DecodeCBORInMapFrom[AdventurerView](data)
}

// AppendDMView writes one dmview in the map shape.
func AppendDMView(dst []byte, v DMView) []byte { return cborbind.AppendCBORInMapTo(dst, v) }

// DecodeDMView reads one dmview.
func DecodeDMView(data []byte) (DMView, error) { return cborbind.DecodeCBORInMapFrom[DMView](data) }
