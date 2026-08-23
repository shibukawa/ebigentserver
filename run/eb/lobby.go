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
	// Background paints the lobby.
	Background color.Color
}

// appHost is what a gathering scene needs from the wrapper. It is one
// interface rather than a pile of callbacks so that a game replacing
// this screen has a single thing to satisfy.
type appHost[S, A, O any] interface {
	// Start finalizes the roster and begins the match.
	Start() error
	// Last reports the previous match, or nil.
	Last() *run.MatchResult
	// Roster is the one being gathered.
	Roster() *run.Roster[S, A, O]
	// Network is the preset, or nil when this build plays offline.
	Matchmaking() run.Matchmaking[S, A, O]
	// Host is the offer this instance already makes, or nil. An
	// instance that hosted the last match still holds the listener and
	// is still announcing, so a second lobby must not go looking — it
	// would find itself.
	Host() run.Host[S, A, O]
	// BecomeHost and BecomeGuest tell the wrapper which part this
	// instance took.
	BecomeHost(run.Host[S, A, O])
	BecomeGuest(run.Guest[S, A, O])
	// Context bounds the work a scene starts.
	Context() context.Context
	// Options is the framework declaration.
	Declared() (run.Options, LobbyOptions, run.Binding[S, A, O])
}

// phase is where the lobby is in gathering.
type phase int

const (
	// phaseAsking is waiting on the network to say who is out there.
	phaseAsking phase = iota
	// phaseChoosing is showing what answered, so a person decides
	// instead of being put into whichever match replied first.
	phaseChoosing
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
type Lobby[S, A, O any] struct {
	app    appHost[S, A, O]
	roster *run.Roster[S, A, O]
	opts   run.Options
	lobby  LobbyOptions
	binder run.Binding[S, A, O]

	phase phase
	guest bool
	err   error

	mu    sync.Mutex
	found []run.Found
	asked bool

	keys []ebiten.Key
	pads []ebiten.GamepadID
	btns []ebiten.GamepadButton
}

// NewLobby builds the default gathering screen for one match.
func NewLobby[S, A, O any](a appHost[S, A, O], roster *run.Roster[S, A, O]) *Lobby[S, A, O] {
	opts, lobby, binder := a.Declared()
	l := &Lobby[S, A, O]{
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
func (l *Lobby[S, A, O]) seatHost() {
	l.phase = phaseSeated
	if _, err := l.roster.SitLocal("player"); err != nil {
		l.err = err
		return
	}
	l.guest = true
}

// ask puts the discovery on its own goroutine: it takes about as long as
// a beacon interval, and a frozen window is not a lobby.
func (l *Lobby[S, A, O]) ask() {
	found, err := l.app.Matchmaking().Discover(l.app.Context())
	l.mu.Lock()
	l.found, l.asked = found, true
	if err != nil {
		l.err = err
	}
	l.mu.Unlock()
}

// Update advances gathering.
func (l *Lobby[S, A, O]) Update() error {
	switch l.phase {
	case phaseAsking:
		return l.updateAsking()
	case phaseChoosing:
		return l.updateChoosing()
	default:
		return l.updateSeated()
	}
}

// updateAsking waits for the network to answer, then either offers the
// list or, finding nobody, hosts and sits down.
func (l *Lobby[S, A, O]) updateAsking() error {
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
func (l *Lobby[S, A, O]) hostAlone() error {
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
	return l.start()
}

// updateChoosing turns a click on a row into a seat on that host, and a
// click on the last row into hosting instead.
func (l *Lobby[S, A, O]) updateChoosing() error {
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
		// That host filled up or went away between the beacon and the
		// click. Say so and let them pick again.
		l.err = err
		l.mu.Lock()
		l.found = append(l.found[:row:row], l.found[row+1:]...)
		l.mu.Unlock()
		return nil
	}
	l.app.BecomeGuest(guest)
	return nil
}

// updateSeated is the offline and the hosting case: take a seat, and
// start when the roster is ready.
func (l *Lobby[S, A, O]) updateSeated() error {
	// NoBots means the empty seats belong to people arriving from
	// elsewhere. When the last of them does, waiting for another press
	// would mean somebody has to be watching the screen to notice —
	// which is the job this scene was meant to do.
	if l.lobby.NoBots && l.guest && l.roster.Complete() {
		return l.start()
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
		if seat.LocalHuman() {
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

// Row geometry for the chooser, in the logical pixels the game declared.
const (
	rowTop    = 44
	rowHeight = 18
)

// pickedRow reports which row a click or a key landed on. Number keys
// work too, so a build that accepts no mouse can still choose.
func (l *Lobby[S, A, O]) pickedRow(rows int) (int, bool) {
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

	switch l.phase {
	case phaseAsking:
		ebitenutil.DebugPrintAt(screen, "looking for a game on this network...", 8, 28)
	case phaseChoosing:
		l.drawChoices(screen)
	default:
		l.drawSeats(screen)
	}
	if l.err != nil {
		ebitenutil.DebugPrintAt(screen, l.err.Error(), 8, rowTop+rowHeight*6)
	}
}

// drawChoices lists what answered, plus the option of hosting instead.
func (l *Lobby[S, A, O]) drawChoices(screen *ebiten.Image) {
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
func (l *Lobby[S, A, O]) drawSeats(screen *ebiten.Image) {
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
func (l *Lobby[S, A, O]) previous() string {
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
func (l *Lobby[S, A, O]) prompt() string {
	if l.guest && l.lobby.NoBots && !l.roster.Complete() {
		return "waiting for another player to join..."
	}
	if l.lobby.Prompt != "" && !l.guest {
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
	if l.guest && !l.lobby.AutoStart {
		return verb + " to start"
	}
	return verb + " to play"
}
