package eb

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/shibukawa/ebigentserver/run"
)

// result is the screen between a finished match and the next lobby.
//
// It exists because the last frame of a match is the one a player most
// wants to look at, and returning to the lobby the instant the session
// says "over" takes it away before they have seen why. So the board
// stays up, drawn by the same client that drew it a moment ago, and the
// dismissal is theirs to make.
type result[S, A, O any] struct {
	app     *app[S, A, O]
	client  Client[S, A, O]
	line    string
	done    bool
	keys    []ebiten.Key
	pads    []ebiten.GamepadID
	btns    []ebiten.GamepadButton
	devices run.Devices
}

// newResult freezes the outcome into a line and holds the last board.
func newResult[S, A, O any](a *app[S, A, O], line string) *result[S, A, O] {
	return &result[S, A, O]{
		app: a, client: a.opts.Client, line: line,
		devices: a.opts.Options.Devices,
	}
}

// Update waits for the player to dismiss the board.
func (r *result[S, A, O]) Update() error {
	if r.done || !r.dismissed() {
		return nil
	}
	r.done = true
	return r.app.gather()
}

// Draw keeps the final board on screen and writes the outcome over it.
func (r *result[S, A, O]) Draw(screen *ebiten.Image) {
	r.client.Draw(screen)
	ebitenutil.DebugPrintAt(screen, r.line, 8, 8)
	ebitenutil.DebugPrintAt(screen, dismissVerb(r.devices), 8, 22)
}

// dismissed reports a press on any accepted device.
func (r *result[S, A, O]) dismissed() bool {
	if r.devices.Has(run.Mouse) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	if r.devices.Has(run.Keyboard) {
		r.keys = inpututil.AppendJustPressedKeys(r.keys[:0])
		if len(r.keys) > 0 {
			return true
		}
	}
	if r.devices.Has(run.Gamepad) {
		r.pads = ebiten.AppendGamepadIDs(r.pads[:0])
		for _, id := range r.pads {
			r.btns = inpututil.AppendJustPressedGamepadButtons(id, r.btns[:0])
			if len(r.btns) > 0 {
				return true
			}
		}
	}
	return false
}

func dismissVerb(d run.Devices) string {
	switch {
	case d.Has(run.Mouse):
		return "click to continue"
	case d.Has(run.Keyboard):
		return "press any key to continue"
	default:
		return "press a button to continue"
	}
}

// outcomeLine says how it went from the local player's side, which is
// the only side they can check against what they just watched.
func outcomeLine(res *run.MatchResult) string {
	if res == nil {
		return "match over"
	}
	if res.Err != nil {
		return "match ended: " + res.Err.Error()
	}
	for _, seat := range res.Seats {
		if seat.Kind != run.LocalHuman {
			continue
		}
		if sig, ok := res.Outcome(seat.Slot); ok {
			return "you " + sig.Terminal.String()
		}
	}
	return "match over"
}
