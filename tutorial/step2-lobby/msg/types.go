// Package msg declares tic-tac-toe's wire types.
//
// Step 1 kept the board in an ordinary Go array and never had to say how
// it travelled. Once a second machine is watching the same board, the
// shape of those bytes becomes a contract between two builds, so it is
// declared once here and generated rather than written by hand.
package msg

import (
	"github.com/shibukawa/tinybind-go/cborbind"
)

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

// Move is data:player-input: the cell a player claims.
type Move struct {
	// Cell is 0..8, row-major from the top-left.
	Cell uint8 `json:"cell"`
}

// TTTWorld is the authoritative board on the world profile.
type TTTWorld struct {
	// Cells is the grid, 9 entries: 0 empty, 1 X, 2 O.
	Cells []uint8 `json:"cells"`
	// Turn is the slot to move, 0 once the game is over.
	Turn uint16 `json:"turn"`
	// Winner is the winning slot, 0 on a draw or an open game.
	Winner uint16 `json:"winner"`
	// Line is the winning triple, meaningful only when Winner is set.
	Line []uint8 `json:"line"`
	// Moves counts marks placed.
	Moves uint8 `json:"moves"`
	// Over marks a finished game, won or drawn.
	Over bool `json:"over"`
}

// The calls below are what ask the generator for each codec: there is no
// declaration to write any more, and naming an entry point is the ask
// (requirement:cborbind-migration).
//
// Which container a type uses is a contract rather than a preference. An
// input is an array — positional, no field names on the wire, and both
// ends rebuilt together — which is concept:cbor-wire-profile. The board is
// a map, so a decoder can skip a key it does not know and the two ends may
// ship apart, which is concept:cbor-world-profile.

// AppendMove writes one move in the array shape.
func AppendMove(dst []byte, v Move) []byte { return cborbind.AppendCBORInArrayTo(dst, v) }

// DecodeMove reads one move.
func DecodeMove(data []byte) (Move, error) { return cborbind.DecodeCBORInArrayFrom[Move](data) }

// AppendTTTState writes one board in the map shape.
func AppendTTTState(dst []byte, v TTTWorld) []byte { return cborbind.AppendCBORInMapTo(dst, v) }

// DecodeTTTState reads one board.
func DecodeTTTState(data []byte) (TTTWorld, error) {
	return cborbind.DecodeCBORInMapFrom[TTTWorld](data)
}
