// Command step1-hotseat is tic-tac-toe for two people sharing one mouse.
//
//	go run ./tutorial/step1-hotseat
//
// It is the starting point of the tutorial and it uses no part of
// ebigentserver: an ordinary Ebitengine program, the kind anybody would
// write first. Later steps take this same board and give it a session, a
// second machine, and a recorded corpus to learn from — so what matters
// here is not what it does but where its seams already are.
//
// Two seams are worth naming now, because every later step pushes on
// them:
//
//   - cellAt is the whole of the input handling. A pixel becomes a cell
//     index here, and nothing below this line knows what a mouse is.
//   - package game holds the rules and does not import Ebitengine, so
//     the rules can be tested without opening a window.
//
// Everything else in this file is presentation: it reads state and draws
// it, and decides nothing.
package main

import (
	"fmt"
	"image/color"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shibukawa/ebigentserver/tutorial/step1-hotseat/game"
)

// Board geometry in logical pixels. Ebitengine scales the logical screen
// up to the window, so every number below is resolution independent.
const (
	cellPx  = 100
	margin  = 10
	boardPx = cellPx * 3
	screenW = boardPx + margin*2
	screenH = screenW + 40 // room for the status line
	scale   = 2
)

// noCell is the hover value when the cursor is off the board.
const noCell = -1

func main() {
	ebiten.SetWindowSize(screenW*scale, screenH*scale)
	ebiten.SetWindowTitle("tic-tac-toe - step 1: hot seat")
	if err := ebiten.RunGame(&app{state: game.New(), hover: noCell}); err != nil {
		fatal(err)
	}
}

// app is the Ebitengine side. It holds the game state and the cursor,
// and no rules of its own.
type app struct {
	state game.State
	// hover is the cell under the cursor, or noCell.
	hover int
}

// cellAt maps a cursor position to a board cell — the one place a pixel
// turns into a move.
func cellAt(x, y int) (int, bool) {
	x, y = x-margin, y-margin
	if x < 0 || y < 0 || x >= boardPx || y >= boardPx {
		return noCell, false
	}
	return (y/cellPx)*3 + x/cellPx, true
}

// Update reads the mouse and hands the rules a cell. Ebitengine calls it
// at a fixed rate; Draw runs on its own schedule, which is why input is
// read here and only here.
func (a *app) Update() error {
	mx, my := ebiten.CursorPosition()

	a.hover = noCell
	if cell, ok := cellAt(mx, my); ok && a.state.Legal(cell) {
		a.hover = cell
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if a.state.Over {
			a.state = game.New() // any click starts the next game
		} else if cell, ok := cellAt(mx, my); ok {
			// Place refuses an occupied cell on its own, so a click
			// anywhere on the board is safe to pass straight through.
			a.state.Place(cell)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		a.state = game.New()
	}
	return nil
}

var (
	colBG      = color.RGBA{0x1c, 0x20, 0x26, 0xff}
	colGrid    = color.RGBA{0x3c, 0x45, 0x50, 0xff}
	colX       = color.RGBA{0x6f, 0xb1, 0xf0, 0xff}
	colO       = color.RGBA{0xf0, 0xa8, 0x6f, 0xff}
	colWinLine = color.RGBA{0xf2, 0xe6, 0x8a, 0xff}
	hoverX     = color.RGBA{0x24, 0x33, 0x45, 0xff}
	hoverO     = color.RGBA{0x3a, 0x2e, 0x24, 0xff}
)

// Draw renders the state. Nothing here decides anything: if the picture
// is wrong, the rules are wrong.
func (a *app) Draw(screen *ebiten.Image) {
	screen.Fill(colBG)

	// The cell under the cursor, tinted with whoever is about to play.
	if a.hover != noCell {
		x, y := cellOrigin(a.hover)
		tint := hoverX
		if a.state.Turn == game.O {
			tint = hoverO
		}
		vector.DrawFilledRect(screen, x, y, cellPx, cellPx, tint, false)
	}

	// The two inner lines each way.
	for i := 1; i < 3; i++ {
		v := float32(margin + i*cellPx)
		lo, hi := float32(margin), float32(margin+boardPx)
		vector.StrokeLine(screen, lo, v, hi, v, 2, colGrid, true)
		vector.StrokeLine(screen, v, lo, v, hi, 2, colGrid, true)
	}

	const radius = cellPx * 0.28
	for cell, mark := range a.state.Board {
		cx, cy := cellCenter(cell)
		switch mark {
		case game.X:
			vector.StrokeLine(screen, cx-radius, cy-radius, cx+radius, cy+radius, 6, colX, true)
			vector.StrokeLine(screen, cx+radius, cy-radius, cx-radius, cy+radius, 6, colX, true)
		case game.O:
			vector.StrokeCircle(screen, cx, cy, radius, 6, colO, true)
		}
	}

	if a.state.Winner != game.Empty {
		ax, ay := cellCenter(a.state.Line[0])
		bx, by := cellCenter(a.state.Line[2])
		vector.StrokeLine(screen, ax, ay, bx, by, 5, colWinLine, true)
	}

	// The debug font covers Latin-1 only, so the status line is ASCII.
	ebitenutil.DebugPrintAt(screen, a.status(), margin, margin+boardPx+12)
}

// status is the one line under the board.
func (a *app) status() string {
	switch {
	case a.state.Winner != game.Empty:
		return fmt.Sprintf("%s wins - click to play again", a.state.Winner)
	case a.state.Over:
		return "draw - click to play again"
	default:
		return fmt.Sprintf("%s to move", a.state.Turn)
	}
}

// Layout fixes the logical resolution; Ebitengine scales it to the window.
func (a *app) Layout(int, int) (int, int) { return screenW, screenH }

// cellOrigin returns a cell's top-left logical pixel.
func cellOrigin(cell int) (x, y float32) {
	return float32(margin + (cell%3)*cellPx), float32(margin + (cell/3)*cellPx)
}

// cellCenter returns a cell's middle logical pixel.
func cellCenter(cell int) (x, y float32) {
	x, y = cellOrigin(cell)
	return x + cellPx/2, y + cellPx/2
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "step1-hotseat:", err)
	os.Exit(1)
}
