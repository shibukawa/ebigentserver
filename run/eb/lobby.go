package eb

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/shibukawa/ebigentserver/run"
)

// LobbyOptions configures the default ui:lobby-scene.
type LobbyOptions struct {
	// Prompt replaces the default instruction line.
	Prompt string
	// AutoStart starts the match as soon as this machine holds every
	// local seat it is allowed. A solo game sets it, so one press both
	// takes the seat and begins; a game expecting several people at one
	// screen leaves it clear so they can all join first.
	AutoStart bool
	// NoBots leaves unfilled seats empty instead of seating agents,
	// which is what a game waiting for remote players wants. Without it
	// the remaining seats are filled from Binding.NewAgent on start —
	// the enemies of a solo game, or a stand-in opponent.
	//
	// It also changes what starts the match: with bots, a press starts
	// it, because the roster is complete the moment this machine is
	// done. Without them the roster completes when the last person
	// arrives, and that is the start signal.
	NoBots bool
	// Background paints the lobby.
	Background color.Color
}

// host is what a gathering scene needs from the wrapper: permission to
// begin, and whatever the previous match came to. The app implements it.
type host interface {
	Start() error
	Last() *run.MatchResult
}

// Lobby is the default gathering screen: it shows the roster, seats a
// local player on an accepted device, and starts the match.
//
// It supplies nothing that api:roster does not, which is the point — a
// game replacing this screen keeps admission, bot seating, and the match
// lifecycle, and loses only these few lines of drawing.
type Lobby[S, A, O any] struct {
	app    host
	roster *run.Roster[S, A, O]
	opts   run.Options
	lobby  LobbyOptions
	binder run.Binding[S, A, O]

	joined bool
	err    error
	keys   []ebiten.Key
	pads   []ebiten.GamepadID
	btns   []ebiten.GamepadButton
}

// NewLobby builds the default lobby over a roster.
func NewLobby[S, A, O any](a *app[S, A, O], roster *run.Roster[S, A, O]) *Lobby[S, A, O] {
	return &Lobby[S, A, O]{
		app:    a,
		roster: roster,
		opts:   a.opts.Options,
		lobby:  a.opts.Lobby,
		binder: a.opts.Binding,
	}
}

// Update reads the accepted devices and advances gathering.
//
// The first press takes a seat. A press once this machine holds every
// local seat it may starts the match — or, with AutoStart, taking the
// last one is itself the start.
func (l *Lobby[S, A, O]) Update() error {
	// NoBots means the empty seats belong to people arriving from
	// elsewhere. When the last of them does, waiting for another press
	// would mean somebody has to be watching the screen to notice —
	// which is the job this scene was meant to do.
	if l.lobby.NoBots && l.joined && l.roster.Complete() {
		return l.start()
	}
	if !l.pressed() {
		return nil
	}
	if l.canJoin() {
		if _, err := l.roster.JoinLocal("player"); err != nil {
			l.err = err
			return nil
		}
		l.joined = true
		if !l.lobby.AutoStart || l.canJoin() {
			return nil
		}
	}
	if !l.joined {
		return nil
	}
	return l.start()
}

// canJoin reports whether another person may sit down at this machine.
func (l *Lobby[S, A, O]) canJoin() bool {
	limit := 1
	if l.opts.MaxLocalSeats > 0 {
		limit = l.opts.MaxLocalSeats
	}
	local := 0
	free := false
	for _, seat := range l.roster.Seats() {
		if seat.Kind == run.LocalHuman {
			local++
		}
		if !seat.Filled() {
			free = true
		}
	}
	return free && local < limit
}

// start fills the remaining seats and hands the roster to the wrapper.
func (l *Lobby[S, A, O]) start() error {
	if !l.lobby.NoBots {
		if err := l.roster.FillBots(l.binder.NewAgent); err != nil {
			l.err = err
			return nil
		}
	}
	for _, seat := range l.roster.Seats() {
		l.roster.SetReady(seat.Slot, true)
	}
	if !l.roster.Ready() {
		// Seats are still open and waiting on somebody else — a
		// remote player arriving through flow:session-admission, or
		// another person at this screen.
		return nil
	}
	return l.app.Start()
}

// pressed reports a start signal on any accepted device. Only devices the
// game declared are read, so a keyboard-only game never reports a
// gamepad press it has no adapter for.
func (l *Lobby[S, A, O]) pressed() bool {
	if l.opts.Devices.Has(run.Keyboard) {
		l.keys = inpututil.AppendJustPressedKeys(l.keys[:0])
		if len(l.keys) > 0 {
			return true
		}
	}
	if l.opts.Devices.Has(run.Mouse) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	if l.opts.Devices.Has(run.Gamepad) {
		l.pads = ebiten.AppendGamepadIDs(l.pads[:0])
		for _, id := range l.pads {
			l.btns = inpututil.AppendJustPressedGamepadButtons(id, l.btns[:0])
			if len(l.btns) > 0 {
				return true
			}
		}
	}
	return false
}

// Draw shows who is seated and what to press.
func (l *Lobby[S, A, O]) Draw(screen *ebiten.Image) {
	bg := l.lobby.Background
	if bg == nil {
		bg = color.RGBA{0x12, 0x16, 0x1c, 0xff}
	}
	screen.Fill(bg)

	title := l.opts.Name
	if title == "" {
		title = "lobby"
	}
	ebitenutil.DebugPrintAt(screen, title, 8, 8)

	y := 28
	for _, seat := range l.roster.Seats() {
		line := fmt.Sprintf("slot %d  %-12s %s", seat.Slot, seat.Kind, seat.ID)
		if !seat.Filled() {
			line = fmt.Sprintf("slot %d  open", seat.Slot)
		}
		ebitenutil.DebugPrintAt(screen, line, 8, y)
		y += 14
	}

	y += 6
	if line := l.previous(); line != "" {
		ebitenutil.DebugPrintAt(screen, line, 8, y)
		y += 14
	}
	ebitenutil.DebugPrintAt(screen, l.prompt(), 8, y)
	if l.err != nil {
		ebitenutil.DebugPrintAt(screen, l.err.Error(), 8, y+14)
	}
}

// previous describes how the last match went, so returning to the lobby
// reports a result instead of looking like a restart. It reads the seat a
// person held, since that is the outcome they care about.
func (l *Lobby[S, A, O]) previous() string {
	last := l.app.Last()
	if last == nil {
		return ""
	}
	for _, seat := range last.Seats {
		if seat.Kind != run.LocalHuman {
			continue
		}
		if sig, ok := last.Outcome(seat.Slot); ok {
			return fmt.Sprintf("last match: %s after %d ticks", sig.Terminal, last.Ticks)
		}
	}
	return fmt.Sprintf("last match: %d ticks", last.Ticks)
}

// prompt is the instruction line, derived from the accepted devices so it
// never tells a player to press something this build does not read.
func (l *Lobby[S, A, O]) prompt() string {
	if l.lobby.Prompt != "" {
		return l.lobby.Prompt
	}
	verb := "press start"
	switch {
	case l.opts.Devices.Has(run.Keyboard):
		verb = "press any key"
	case l.opts.Devices.Has(run.Mouse):
		verb = "click"
	case l.opts.Devices.Has(run.Gamepad):
		verb = "press a gamepad button"
	}
	if l.joined && !l.lobby.AutoStart {
		return verb + " to start"
	}
	return verb + " to play"
}
