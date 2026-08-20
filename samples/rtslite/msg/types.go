// Package msg declares the rts-lite wire types: the two CBOR profiles
// side by side in one game. Commands ride the wire profile
// (concept:cbor-wire-profile — fixed-order arrays, the smallest most
// frequent payload); the fog-of-war player views ride the world profile
// (concept:cbor-world-profile — evolvable maps for large structures),
// diffed per receiver.
package msg

import (
	"github.com/shibukawa/tinybind-go/cborbind"
)

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

// Map dimensions in cells.
const (
	MapW = 96
	MapH = 64
)

// UnitID packs the owning slot into the high byte and a per-owner
// sequence below (decision:owner-namespaced-entity-ids in a wire-compact
// form): two owners can never mint the same id, with no coordination.
type UnitID = uint32

// MakeUnitID composes an id from owner slot and per-owner sequence.
func MakeUnitID(owner uint16, seq uint32) UnitID {
	return uint32(owner)<<24 | seq&0xFFFFFF
}

// OwnerOf extracts the owning slot.
func OwnerOf(id UnitID) uint16 { return uint16(id >> 24) }

// Command is data:player-input on the wire profile: one order to one
// unit. A slot streams many of these per tick — the "one slot commands
// many units" structure of sample:rts-lite.
type Command struct {
	Tick uint32
	// Unit is the ordered unit.
	Unit uint32
	// TargetX, TargetY is the move destination.
	TargetX uint8
	TargetY uint8
}

var _ = cborbind.GenerateWireCodec[Command]()

// Unit is one combat unit in the full state.
type Unit struct {
	ID    uint32 `cbor:"id,key=1,identity"`
	X     uint8  `cbor:"x,key=2"`
	Y     uint8  `cbor:"y,key=3"`
	TX    uint8  `cbor:"tx,key=4"`
	TY    uint8  `cbor:"ty,key=5"`
	HP    int8   `cbor:"hp,key=6"`
	Alive bool   `cbor:"alive,key=7"`
}

// RTSState is the authoritative world: large enough that sending it
// whole every tick is visibly the wrong answer.
type RTSState struct {
	Tick      uint64 `cbor:"tick,key=1"`
	Units     []Unit `cbor:"units,key=2"`
	NextSeq   uint32 `cbor:"next_seq,key=3"`
	TickLimit uint32 `cbor:"limit,key=4"`
	Over      bool   `cbor:"over,key=5"`
	// Winner is the surviving slot, 0 while playing or on a draw.
	Winner uint16 `cbor:"winner,key=6"`
}

var _ = cborbind.GenerateWorldDelta[RTSState]()

// Glimpse is an enemy unit as fog of war lets a player see it: identity,
// position, owner — no orders, no exact health.
type Glimpse struct {
	ID    uint32 `cbor:"id,key=1,identity"`
	X     uint8  `cbor:"x,key=2"`
	Y     uint8  `cbor:"y,key=3"`
	Owner uint16 `cbor:"owner,key=4"`
}

// PlayerView is one player's fog-of-war projection: own units in full,
// enemies only inside sight range of some own unit (interest management
// through concept:agent-view — a player receives a fraction of the map).
type PlayerView struct {
	Tick uint64 `cbor:"tick,key=1"`
	You  uint16 `cbor:"you,key=2"`
	// Own is every unit the player commands, in full.
	Own []Unit `cbor:"own,key=3"`
	// Visible is the fraction of enemy units inside sight.
	Visible []Glimpse `cbor:"visible,key=4"`
	// OwnAlive and EnemyAlive give the scoped score material: total
	// enemy count is public knowledge in this game (army sizes are
	// announced), positions are not.
	OwnAlive   uint16 `cbor:"own_alive,key=5"`
	EnemyAlive uint16 `cbor:"enemy_alive,key=6"`
	Over       bool   `cbor:"over,key=7"`
	Winner     uint16 `cbor:"winner,key=8"`
}

var _ = cborbind.GenerateWorldDelta[PlayerView]()
