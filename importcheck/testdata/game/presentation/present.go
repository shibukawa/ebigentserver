// Package presentation may import the engine: it is rendering code.
package presentation

import "github.com/hajimehoshi/ebiten/v2"

// Draw uses the engine.
func Draw() { ebiten.RunGame() }
