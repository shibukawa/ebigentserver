package eb

import (
	"context"
	"fmt"
	"image/color"
	"sync"

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
	// StandIn offers a bot for the seats still open, taken by a press
	// rather than seated at start. It is for a game whose empty seats
	// belong to people — NoBots — but whose player is entitled to stop
	// waiting for one.
	//
	// Step 2's rule was that a question with one answer is not worth
	// asking, which is why hosting alone costs no press. This is the
	// same rule read forwards: once there are two answers — wait, or
	// play the stand-in — the press is worth asking for.
	//
	// It needs Binding.NewAgent; FillBots reports its absence.
	StandIn bool
	// Background paints the lobby.
	Background color.Color
}

// appHost is what a gathering scene needs from the wrapper. It is one
// interface rather than a pile of callbacks so that a game replacing
// this screen has a single thing to satisfy.
type appHost[W, A, S any] interface {
	// Start finalizes the roster and begins the match.
	Start() error
	// Last reports the previous match, or nil.
	Last() *run.MatchResult
	// Roster is the one being gathered.
	Roster() *run.Roster[W, A, S]
	// Network is the preset, or nil when this build plays offline.
	Matchmaking() run.Matchmaking[W, A, S]
	// Host is the offer this instance already makes, or nil. An
	// instance that hosted the last match still holds the listener and
	// is still announcing, so a second lobby must not go looking — it
	// would find itself.
	Host() run.Host[W, A, S]
	// BecomeHost and BecomeGuest tell the wrapper which part this
	// instance took.
	BecomeHost(run.Host[W, A, S])
	BecomeGuest(run.Guest[W, A, S])
	// Context bounds the work a scene starts.
	Context() context.Context
	// Options is the framework declaration.
	Declared() (run.Options, LobbyOptions, run.Binding[W, A, S])
}

// phase is where the lobby is in gathering.
type phase int

const (
	// phaseAsking is waiting on the network to say who is out there.
	phaseAsking phase = iota
	// phaseChoosing is showing what answered, so a person decides
	// instead of being put into whichever match replied first.
	phaseChoosing
	// phaseLooking is inside a room but holding no seat: the roster is
	// on screen and the player may sit or go back. Going back frees
	// nothing, which is why this state is worth having.
	phaseLooking
	// phaseSeated is holding a seat: hosting and waiting, or gathering
	// people at this screen.
	phaseSeated
)

// Lobby is the default gathering screen. With a Matchmaking it asks the
// network first and lets the player pick; without one it goes straight
// to seating people at this machine.
//
// It supplies nothing that api:roster does not, which is the point — a
// game replacing this screen keeps admission, bot seating, and the match
// lifecycle, and loses only these few lines of drawing.
type Lobby[W, A, S any] struct {
	app    appHost[W, A, S]
	roster *run.Roster[W, A, S]
	opts   run.Options
	lobby  LobbyOptions
	binder run.Binding[W, A, S]

	phase   phase
	guest   bool
	looking run.Guest[W, A, S]
	err     error

	mu    sync.Mutex
	found []run.Found
	asked bool

	keys []ebiten.Key
	pads []ebiten.GamepadID
	btns []ebiten.GamepadButton
}

// NewLobby builds the default gathering screen for one match.
func NewLobby[W, A, S any](a appHost[W, A, S], roster *run.Roster[W, A, S]) *Lobby[W, A, S] {
	opts, lobby, binder := a.Declared()
	l := &Lobby[W, A, S]{
		app: a, roster: roster,
		opts: opts, lobby: lobby, binder: binder,
		phase: phaseSeated,
	}
	switch {
	case a.Host() != nil:
		// Already offering: the next match gathers on the same terms
		// as the last one, so sit down and wait again.
		l.seatHost()
	case a.Matchmaking() != nil:
		l.phase = phaseAsking
		go l.ask()
	}
	return l
}

// seatHost takes the local seat of a hosting instance. It is separate
// from hostAlone because the offer may already exist.
func (l *Lobby[W, A, S]) seatHost() {
	l.phase = phaseSeated
	if _, err := l.roster.SitLocal("player"); err != nil {
		l.err = err
		return
	}
	l.guest = true
}

// ask puts the discovery on its own goroutine: it takes about as long as
// a beacon interval, and a frozen window is not a lobby.
func (l *Lobby[W, A, S]) ask() {
	found, err := l.app.Matchmaking().Discover(l.app.Context())
	l.mu.Lock()
	l.found, l.asked = found, true
	if err != nil {
		l.err = err
	}
	l.mu.Unlock()
}

// Update advances gathering.
func (l *Lobby[W, A, S]) Update() error {
	switch l.phase {
	case phaseAsking:
		return l.updateAsking()
	case phaseChoosing:
		return l.updateChoosing()
	case phaseLooking:
		return l.updateLooking()
	default:
		return l.updateSeated()
	}
}

// updateAsking waits for the network to answer, then either offers the
// list or, finding nobody, hosts and sits down.
func (l *Lobby[W, A, S]) updateAsking() error {
	l.mu.Lock()
	asked, found := l.asked, l.found
	l.mu.Unlock()
	if !asked {
		return nil
	}
	if len(found) > 0 {
		l.phase = phaseChoosing
		return nil
	}
	return l.hostAlone()
}

// hostAlone is what happens when nobody answers: this instance offers
// its own match and takes its seat without being asked to.
//
// Being told to press a key here would be asking a question with one
// answer — there is nobody else to pick, and the seat is the only thing
// on offer. So the press is spent later, on the choice that has two
// sides: whether to join somebody.
func (l *Lobby[W, A, S]) hostAlone() error {
	hosting, err := l.app.Matchmaking().Host(l.app.Context(), l.roster, 0)
	if err != nil {
		l.err = err
		l.phase = phaseSeated
		return nil
	}
	l.app.BecomeHost(hosting)
	l.seatHost()
	if l.err != nil {
		return nil
	}
	return l.start(!l.lobby.NoBots)
}

// updateChoosing turns a click on a row into a seat on that host, and a
// click on the last row into hosting instead.
func (l *Lobby[W, A, S]) updateChoosing() error {
	l.mu.Lock()
	found := l.found
	l.mu.Unlock()

	row, ok := l.pickedRow(len(found) + 1)
	if !ok {
		return nil
	}
	if row == len(found) {
		return l.hostAlone()
	}
	guest, err := l.app.Matchmaking().Match(l.app.Context(), found[row])
	if err != nil {
		// That host went away between the beacon and the click. Say so
		// and let them pick again.
		l.err = err
		l.mu.Lock()
		l.found = append(l.found[:row:row], l.found[row+1:]...)
		l.mu.Unlock()
		return nil
	}
	// Reaching a room is not taking a seat. Show who is there and let
	// the player decide, which is the whole reason the two are separate.
	l.looking, l.phase, l.err = guest, phaseLooking, nil
	return nil
}

// updateLooking is inside a room with no seat yet: sit, or go back to the
// list. Going back costs the room nothing.
func (l *Lobby[W, A, S]) updateLooking() error {
	if l.back() {
		_ = l.looking.Close()
		l.looking, l.phase = nil, phaseChoosing
		return nil
	}
	if !l.pressed() {
		return nil
	}
	if err := l.looking.Sit(l.app.Context()); err != nil {
		// Somebody took the last seat while this player was reading the
		// roster, which is exactly the race looking makes visible.
		l.err = err
		return nil
	}
	l.app.BecomeGuest(l.looking)
	l.looking = nil
	return nil
}

// updateSeated is the offline and the hosting case: take a seat, and
// start when the roster is ready.
func (l *Lobby[W, A, S]) updateSeated() error {
	// NoBots means the empty seats belong to people arriving from
	// elsewhere. When the last of them does, waiting for another press
	// would mean somebody has to be watching the screen to notice —
	// which is the job this scene was meant to do.
	if l.lobby.NoBots && l.guest && l.roster.Complete() {
		return l.start(false)
	}
	if !l.pressed() {
		return nil
	}
	if l.canJoin() {
		if _, err := l.roster.SitLocal("player"); err != nil {
			l.err = err
			return nil
		}
		l.guest = true
		if !l.lobby.AutoStart || l.canJoin() {
			return nil
		}
	}
	if !l.guest {
		return nil
	}
	// The press means "go" either way. What it seats differs: a solo
	// game's enemies were always this machine's to fill, and a
	// stand-in is the player saying they are done waiting for the
	// person whose seat it is.
	return l.start(!l.lobby.NoBots || l.lobby.StandIn)
}

// canJoin reports whether another person may sit down at this machine.
func (l *Lobby[W, A, S]) canJoin() bool {
	limit := 1
	if l.opts.MaxLocalSeats > 0 {
		limit = l.opts.MaxLocalSeats
	}
	local := 0
	free := false
	for _, seat := range l.roster.Seats() {
		if seat.LocalHuman() {
			local++
		}
		if !seat.Filled() {
			free = true
		}
	}
	return free && local < limit
}

// start hands the roster to the wrapper, filling what is still open
// first when bots says it may. A roster that is still short of a person
// after that is not an error: the lobby simply keeps waiting.
func (l *Lobby[W, A, S]) start(bots bool) error {
	if bots {
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

// back reports a request to leave without sitting. Escape is the one key
// read regardless of the declared devices: a player who reached a room
// they do not want has to be able to get out of it, and a game that
// accepts only a gamepad still runs this screen on a keyboard.
func (l *Lobby[W, A, S]) back() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeyEscape)
}

// drawRoom shows the roster of a room this instance reached but has not
// sat in. It refreshes on its own, so somebody arriving appears here
// while the player is still reading.
func (l *Lobby[W, A, S]) drawRoom(screen *ebiten.Image) {
	room := l.looking.Room()
	title := room.Title
	if title == "" {
		title = "this room"
	}
	ebitenutil.DebugPrintAt(screen, "in "+title+" - press to sit, esc to go back", 8, 28)
	for i, seat := range room.Seats {
		who := "empty"
		if seat.Filled() {
			who = seat.Kind.String()
			if seat.ID != "" {
				who += " (" + seat.ID + ")"
			}
		}
		ebitenutil.DebugPrintAt(screen,
			fmt.Sprintf("  seat %d  %s", seat.Slot, who), 8, rowTop+rowHeight*i)
	}
}

// pressed reports a start signal on any accepted device. Only devices the
// game declared are read, so a keyboard-only game never reports a
// gamepad press it has no adapter for.
func (l *Lobby[W, A, S]) pressed() bool {
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

// Row geometry for the chooser, in the logical pixels the game declared.
const (
	rowTop    = 44
	rowHeight = 18
)

// pickedRow reports which row a click or a key landed on. Number keys
// work too, so a build that accepts no mouse can still choose.
func (l *Lobby[W, A, S]) pickedRow(rows int) (int, bool) {
	if l.opts.Devices.Has(run.Mouse) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		_, y := ebiten.CursorPosition()
		row := (y - rowTop) / rowHeight
		if row >= 0 && row < rows {
			return row, true
		}
	}
	if l.opts.Devices.Has(run.Keyboard) {
		for i := 0; i < rows && i < 9; i++ {
			if inpututil.IsKeyJustPressed(ebiten.Key1 + ebiten.Key(i)) {
				return i, true
			}
		}
	}
	return 0, false
}

// Draw shows what the lobby is doing and what it is waiting for.
func (l *Lobby[W, A, S]) Draw(screen *ebiten.Image) {
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

	switch l.phase {
	case phaseAsking:
		ebitenutil.DebugPrintAt(screen, "looking for a game on this network...", 8, 28)
	case phaseChoosing:
		l.drawChoices(screen)
	case phaseLooking:
		l.drawRoom(screen)
	default:
		l.drawSeats(screen)
	}
	if l.err != nil {
		ebitenutil.DebugPrintAt(screen, l.err.Error(), 8, rowTop+rowHeight*6)
	}
}

// drawChoices lists what answered, plus the option of hosting instead.
func (l *Lobby[W, A, S]) drawChoices(screen *ebiten.Image) {
	l.mu.Lock()
	found := l.found
	l.mu.Unlock()

	ebitenutil.DebugPrintAt(screen, "games on this network - click one to join", 8, 26)
	y := rowTop
	for i, f := range found {
		ebitenutil.DebugPrintAt(screen,
			fmt.Sprintf("%d) %-14s %-21s %d seated", i+1, f.Name, f.Address, f.Players), 8, y)
		y += rowHeight
	}
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d) host your own instead", len(found)+1), 8, y)
}

// drawSeats shows the roster and what it is waiting for.
func (l *Lobby[W, A, S]) drawSeats(screen *ebiten.Image) {
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
}

// previous describes how the last match went, so returning to the lobby
// reports a result instead of looking like a restart. It reads the seat a
// person held, since that is the outcome they care about.
func (l *Lobby[W, A, S]) previous() string {
	last := l.app.Last()
	if last == nil {
		return ""
	}
	for _, seat := range last.Seats {
		if !seat.LocalHuman() {
			continue
		}
		if sig, ok := last.Outcome(seat.Slot); ok {
			return fmt.Sprintf("last match: %s after %d ticks", sig.Terminal, last.Ticks)
		}
	}
	return fmt.Sprintf("last match: %d ticks", last.Ticks)
}

// prompt is the instruction line, derived from what the lobby is
// actually waiting for.
func (l *Lobby[W, A, S]) prompt() string {
	if l.guest && l.lobby.NoBots && !l.roster.Complete() {
		if l.lobby.StandIn {
			return "waiting for another player - " + l.verb() + " to play the bot instead"
		}
		return "waiting for another player to join..."
	}
	if l.lobby.Prompt != "" && !l.guest {
		return l.lobby.Prompt
	}
	if l.guest && !l.lobby.AutoStart {
		return l.verb() + " to start"
	}
	return l.verb() + " to play"
}

// verb names the start signal in terms of a device the game declared, so
// the instruction line never asks for input this build does not read.
func (l *Lobby[W, A, S]) verb() string {
	switch {
	case l.opts.Devices.Has(run.Keyboard):
		return "press any key"
	case l.opts.Devices.Has(run.Mouse):
		return "click"
	case l.opts.Devices.Has(run.Gamepad):
		return "press a gamepad button"
	}
	return "press start"
}
