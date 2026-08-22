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
	Cell uint8 `cbor:"cell,key=1"`
}

var _ = cborbind.GenerateWireCodec[Move]()

// TTTState is the authoritative board on the world profile.
type TTTState struct {
	// Cells is the grid, 9 entries: 0 empty, 1 X, 2 O.
	Cells []uint8 `cbor:"cells,key=1"`
	// Turn is the slot to move, 0 once the game is over.
	Turn uint16 `cbor:"turn,key=2"`
	// Winner is the winning slot, 0 on a draw or an open game.
	Winner uint16 `cbor:"winner,key=3"`
	// Line is the winning triple, meaningful only when Winner is set.
	Line []uint8 `cbor:"line,key=4"`
	// Moves counts marks placed.
	Moves uint8 `cbor:"moves,key=5"`
	// Over marks a finished game, won or drawn.
	Over bool `cbor:"over,key=6"`
}

var _ = cborbind.GenerateWorldDelta[TTTState]()
