// Package rules violates the boundary on purpose: game rules must never
// import the engine.
package rules

import "github.com/hajimehoshi/ebiten/v2"

// Step leaks a rendering dependency into the simulation.
func Step() { ebiten.RunGame() }
