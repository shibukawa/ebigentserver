// Package run is api:run-wrapper, the half of it that links no engine: the
// options a game declares, api:roster, and the match loop of
// concept:match-lifecycle.
//
// It exists because an entry point used to build everything before play
// began — session, every agent, every transport — which only works while
// the roster is known at startup. A real game learns who is playing after
// it launches, so the session moved from process lifetime to match
// lifetime and this package owns the loop around it: gather a roster,
// finalize it into a Match, run, report, gather again.
//
// The engine half lives in run/eb and is the only part that imports
// Ebitengine (rule:engine-import-confined-to-client-entry). A dedicated
// server links this package and not that one, which is how a headless
// build drops the renderer at link time rather than by a flag.
//
// What is deliberately not here: transport choice, topology, and
// synchronization mode, which are run values read from data:run-config
// (rule:transport-selected-by-capability, rule:build-tag-only-for-linkage).
// An option in this package would be the same mistake in another form.
package run

import (
	"errors"
	"fmt"

	"github.com/shibukawa/ebigentserver/session"
)

// Devices is the input device set a build accepts, declared because a
// game cannot accept a device it never wrote an api:input-adapter for.
// It bounds how a player joins in the lobby as well as how they play.
type Devices uint8

const (
	// Keyboard accepts key presses.
	Keyboard Devices = 1 << iota
	// Mouse accepts cursor and button input.
	Mouse
	// Gamepad accepts gamepad buttons and sticks.
	Gamepad
)

// Has reports whether d includes every device in want.
func (d Devices) Has(want Devices) bool { return d&want == want }

// String names the set for logs and lobby prompts.
func (d Devices) String() string {
	var out []byte
	add := func(bit Devices, name string) {
		if d&bit == 0 {
			return
		}
		if len(out) > 0 {
			out = append(out, '+')
		}
		out = append(out, name...)
	}
	add(Keyboard, "keyboard")
	add(Mouse, "mouse")
	add(Gamepad, "gamepad")
	if len(out) == 0 {
		return "none"
	}
	return string(out)
}

// Options is what a game declares to the wrapper that its rules do not
// already say. Everything here is a build fact; anything that can differ
// between two launches of the same artifact belongs in data:run-config.
type Options struct {
	// Name identifies the game in session IDs, episode headers, and the
	// window title.
	Name string
	// Devices is the accepted input device set.
	Devices Devices
	// SharedScreen means several seats read one screen — the shared
	// arrangement of concept:view-arrangement. It decides whether the
	// lobby lets a second local player join at this machine.
	SharedScreen bool
	// MaxLocalSeats bounds how many seats may sit at this machine. 0
	// means one, and a value above one requires SharedScreen.
	MaxLocalSeats int
}

// Validate rejects a declaration that cannot be honored.
func (o Options) Validate() error {
	if o.Name == "" {
		return errors.New("run: Options.Name is required")
	}
	if o.Devices == 0 {
		return fmt.Errorf("run: %s declares no input devices; a game that nobody can control is a configuration error", o.Name)
	}
	if o.MaxLocalSeats < 0 {
		return fmt.Errorf("run: MaxLocalSeats must not be negative, got %d", o.MaxLocalSeats)
	}
	if o.MaxLocalSeats > 1 && !o.SharedScreen {
		return fmt.Errorf("run: %s allows %d local seats without SharedScreen; several seats at one machine share its screen by definition", o.Name, o.MaxLocalSeats)
	}
	return nil
}

// localSeatLimit is MaxLocalSeats with its zero value resolved.
func (o Options) localSeatLimit() int {
	if o.MaxLocalSeats <= 0 {
		return 1
	}
	return o.MaxLocalSeats
}

// Errors the roster and match report.
var (
	// ErrNoFreeSeat means every declared slot is taken.
	ErrNoFreeSeat = errors.New("run: no free seat")
	// ErrSeatTaken means that specific slot is already filled.
	ErrSeatTaken = errors.New("run: seat already taken")
	// ErrUnknownSlot means the slot is not in the declared set.
	ErrUnknownSlot = errors.New("run: unknown slot")
	// ErrLocalSeatLimit means this machine already holds as many seats
	// as the options allow.
	ErrLocalSeatLimit = errors.New("run: local seat limit reached")
	// ErrIncomplete means finalize was called with seats still empty.
	ErrIncomplete = errors.New("run: roster still has empty seats")
)

// slotsEqual reports whether two slot sets hold the same ids.
func slotsEqual(a, b []session.SlotID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
