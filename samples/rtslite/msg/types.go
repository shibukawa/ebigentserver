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

// Unit is one combat unit in the full state.
type Unit struct {
	ID    uint32 `json:"id"`
	X     uint8  `json:"x"`
	Y     uint8  `json:"y"`
	TX    uint8  `json:"tx"`
	TY    uint8  `json:"ty"`
	HP    int8   `json:"hp"`
	Alive bool   `json:"alive"`
}

// RTSState is the authoritative world: large enough that sending it
// whole every tick is visibly the wrong answer.
type RTSState struct {
	Tick      uint64 `json:"tick"`
	Units     []Unit `json:"units"`
	NextSeq   uint32 `json:"next_seq"`
	TickLimit uint32 `json:"limit"`
	Over      bool   `json:"over"`
	// Winner is the surviving slot, 0 while playing or on a draw.
	Winner uint16 `json:"winner"`
}

// Glimpse is an enemy unit as fog of war lets a player see it: identity,
// position, owner — no orders, no exact health.
type Glimpse struct {
	ID    uint32 `json:"id"`
	X     uint8  `json:"x"`
	Y     uint8  `json:"y"`
	Owner uint16 `json:"owner"`
}

// PlayerView is one player's fog-of-war projection: own units in full,
// enemies only inside sight range of some own unit (interest management
// through concept:agent-view — a player receives a fraction of the map).
type PlayerView struct {
	Tick uint64 `json:"tick"`
	You  uint16 `json:"you"`
	// Own is every unit the player commands, in full.
	Own []Unit `json:"own"`
	// Visible is the fraction of enemy units inside sight.
	Visible []Glimpse `json:"visible"`
	// OwnAlive and EnemyAlive give the scoped score material: total
	// enemy count is public knowledge in this game (army sizes are
	// announced), positions are not.
	OwnAlive   uint16 `json:"own_alive"`
	EnemyAlive uint16 `json:"enemy_alive"`
	Over       bool   `json:"over"`
	Winner     uint16 `json:"winner"`
}

// The view below is still asked for by hand. It is not the rule set's
// world — it is the per-player projection this game synchronises
// alongside it — so no rule set declaration names it and the ask has to
// be written (requirement:stage-declares-its-wire).

// AppendPlayerView writes one playerview in the map shape.
func AppendPlayerView(dst []byte, v PlayerView) []byte { return cborbind.AppendCBORInMapTo(dst, v) }

// DecodePlayerView reads one playerview.
func DecodePlayerView(data []byte) (PlayerView, error) {
	return cborbind.DecodeCBORInMapFrom[PlayerView](data)
}
