// Command solo-client plays solo in a window.
//
// This is the only package in the example that imports Ebitengine
// (rule:engine-import-confined-to-client-entry), and what it does with it
// is small on purpose: read the keys, draw the field. The lobby, the
// roster, the session, the match loop, and the recording all come from
// api:run-wrapper, so main declares a game and hands over.
//
// The three hooks of api:tick-hooks are the three methods below. Intake
// turns a held key into a concept:action (rule:no-engine-input-in-game-
// logic: a raw key never reaches the rules). Apply receives each
// committed world on the session's goroutine. Draw renders and decides
// nothing. Arbitration is the fourth, and it is the session's — it runs
// on its own clock, which is why this game replays exactly and why the
// headless entry ticks the same way.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"os"
	"os/signal"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shibukawa/ebigentserver/examples/solo/game"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/run/eb"
	"github.com/shibukawa/ebigentserver/session"
)

// scale turns game units into pixels. The simulation never knows.
const scale = 3

func main() {
	record := flag.String("record", "", "corpus directory; empty records nothing")
	seed := flag.Uint64("seed", 1, "seed of the first match")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	c := &client{}
	err := eb.Run(ctx, eb.Options[game.State, game.Action, game.Observation]{
		Options: game.Options(),
		Binding: game.Binding(),
		Client:  c,
		Lobby: eb.LobbyOptions{
			// One person plays, so taking the seat is also the start:
			// the enemies are agents this process seats, not people
			// the lobby has to wait for.
			AutoStart: true,
			Prompt:    "press any key to run   (arrows or WASD)",
		},
		Time:         session.Paced,
		Seed:         *seed,
		Record:       run.RecordOptions{Root: *record},
		WindowWidth:  game.FieldW * scale,
		WindowHeight: game.FieldH * scale,
		OnMatch: func(res run.MatchResult) {
			if sig, ok := res.Outcome(game.Player); ok {
				fmt.Printf("match %d: %s after %d ticks\n", res.Index, sig.Terminal, res.Ticks)
			}
			if res.EpisodeDir != "" {
				fmt.Printf("episode written to %s\n", res.EpisodeDir)
			}
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "solo-client:", err)
		os.Exit(1)
	}
}

// client is the play scene: input in, picture out, and no rules of its
// own. If the picture is wrong, the rules are wrong.
type client struct {
	// mu guards world, because Apply runs on the session's goroutine and
	// Draw on Ebitengine's.
	mu    sync.Mutex
	world game.State
	got   bool
	// you is the seat this machine holds, learned from the match.
	you session.SlotID
}

var _ eb.Client[game.State, game.Action, game.Observation] = (*client)(nil)

// Intake submits this frame's direction for every seat at this machine.
//
// It submits every frame, including the direction "stay": under the
// newest-input intake policy a later submission supersedes the earlier
// one, so releasing a key stops rather than coasting.
func (c *client) Intake(match *run.Match[game.State, game.Action, game.Observation]) {
	for _, seat := range match.LocalSeats() {
		c.you = seat.Slot
		match.Submit(seat.Slot, game.Action{Move: readDir()})
	}
}

// Apply keeps the newest committed world for the renderer. State holds no
// slices, so assigning it is a complete copy — nothing here aliases what
// the session keeps mutating.
func (c *client) Apply(_ session.Tick, world *game.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.world, c.got = *world, true
}

// Layout fixes the logical resolution; Ebitengine scales it to the window.
func (c *client) Layout(int, int) (int, int) { return game.FieldW * scale, game.FieldH * scale }

var (
	ground   = color.RGBA{0x1b, 0x20, 0x27, 0xff}
	youColor = color.RGBA{0xf7, 0xd0, 0x51, 0xff}
	// One colour per enemy kind, so the two pursuit styles are legible
	// as styles rather than as two identical dots.
	chaserColor  = color.RGBA{0xe0, 0x6c, 0x75, 0xff}
	flankerColor = color.RGBA{0xc6, 0x8c, 0xf0, 0xff}
)

// Draw renders the field.
func (c *client) Draw(screen *ebiten.Image) {
	screen.Fill(ground)
	c.mu.Lock()
	world, got, you := c.world, c.got, c.you
	c.mu.Unlock()
	if !got {
		return
	}

	for i := range world.Actor {
		slot := session.SlotID(i + 1)
		fill := youColor
		if game.IsEnemy(slot) {
			fill = chaserColor
			if kind, _ := game.NewAgent(slot); kind == game.KindFlanker {
				fill = flankerColor
			}
		}
		a := world.Actor[i]
		vector.DrawFilledCircle(screen,
			float32(a.X.FloorToInt())*scale, float32(a.Y.FloorToInt())*scale,
			float32(game.ActorR)*scale, fill, true)
	}

	left := game.TargetTicks - int(world.Tick)
	if left < 0 {
		left = 0
	}
	msg := fmt.Sprintf("slot %d   survive %3d more ticks", you, left)
	if world.Caught {
		msg = fmt.Sprintf("caught by slot %d at tick %d", world.By, world.Tick)
	}
	ebitenutil.DebugPrintAt(screen, msg, 6, 6)
}

// readDir is the whole of api:input-adapter at this scale: device state
// in, one concept:action out, and no game rule anywhere near it.
func readDir() game.Dir {
	switch {
	case ebiten.IsKeyPressed(ebiten.KeyArrowUp), ebiten.IsKeyPressed(ebiten.KeyW):
		return game.Up
	case ebiten.IsKeyPressed(ebiten.KeyArrowDown), ebiten.IsKeyPressed(ebiten.KeyS):
		return game.Down
	case ebiten.IsKeyPressed(ebiten.KeyArrowLeft), ebiten.IsKeyPressed(ebiten.KeyA):
		return game.Left
	case ebiten.IsKeyPressed(ebiten.KeyArrowRight), ebiten.IsKeyPressed(ebiten.KeyD):
		return game.Right
	}
	return gamepadDir()
}

// gamepadDir reads the first gamepad that reports a standard layout. A
// build whose options did not declare Gamepad never reaches here, because
// the wrapper would not have offered the device.
func gamepadDir() game.Dir {
	const deadzone = 0.4
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		switch {
		case y < -deadzone:
			return game.Up
		case y > deadzone:
			return game.Down
		case x < -deadzone:
			return game.Left
		case x > deadzone:
			return game.Right
		}
	}
	return game.Stay
}
