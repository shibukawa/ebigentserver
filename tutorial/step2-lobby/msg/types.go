// Package msg holds tic-tac-toe's world and its action — the two of the
// rule set's three types that travel between machines.
//
// Step 1 kept the board in an ordinary Go array and never had to say how
// it moved. Once a second machine is watching the same board, the shape
// of those bytes is a contract between two builds. Nothing here declares
// that shape: the rule set declaration in package game names these two
// types, and ebigent generate turns that into the codecs beside this
// file (requirement:stage-declares-its-wire).
package msg

// Move is data:player-input: the cell a player claims.
type Move struct {
	// Cell is 0..8, row-major from the top-left.
	Cell uint8 `json:"cell"`
}

// TTTWorld is the authoritative board, and concept:world-state for
// this stage.
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
