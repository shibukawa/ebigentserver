// The client entry point may import the engine.
package main

import (
	"example.com/game/presentation"
	"example.com/game/session"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	session.Tick()
	presentation.Draw()
	ebiten.RunGame()
}
