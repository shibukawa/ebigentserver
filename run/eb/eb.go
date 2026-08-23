// Package eb is the engine half of api:run-wrapper: the one package that
// imports Ebitengine, wraps its run call, and delegates the main loop to
// scenes.
//
// A game that links this package gets concept:match-lifecycle for free.
// It starts at ui:lobby-scene with an empty api:roster — which is where a
// real game starts, because the players are not known until after launch
// — and moves to the game's own play scene when the roster finalizes.
//
// The play scene is where the game meets api:tick-hooks. Update reads
// devices and submits actions (intake); the session commits them on its
// own clock (arbitrate); the committed world arrives through Apply
// (apply). Draw renders and decides nothing.
//
// Splitting this from package run is what lets a headless build drop the
// renderer at link time: a dedicated server imports run and never this
// (rule:engine-import-confined-to-client-entry).
package eb

import (
	"context"
	"errors"
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// Screen is one screen. The wrapper's own ebiten.Game delegates to
// whichever screen is current, which is the shape Ebitengine games
// already use to change screens.
//
// It is not a Stage. A stage is one rule set — the thing the wider
// industry calls a scene — and this is the display state in front of it:
// the lobby is a screen with no rules, and one stage may show several.
type Screen interface {
	// Update advances the screen by one frame.
	Update() error
	// Draw renders it.
	Draw(screen *ebiten.Image)
}

// Client is the game's play scene: an ebiten.Game with the session
// already attached, split so that each of api:tick-hooks has a place.
type Client[S, A, O any] interface {
	// Intake reads this frame's devices and submits an action for each
	// local seat (rule:no-engine-input-in-game-logic: a raw key becomes
	// a concept:action here, and only here). It runs on Ebitengine's
	// goroutine.
	//
	// The seating is this machine's match when it hosts and its link
	// when it guest somebody else's. A play scene never learns which,
	// because from here the difference is only where Submit ends up.
	Intake(seating run.Controls[A])
	// Apply receives each committed world. It runs on the session's
	// goroutine, not Ebitengine's, so it must copy what Draw will read
	// rather than retaining the pointer — the session keeps mutating
	// what it handed over.
	Apply(tick session.Tick, world *S)
	// Draw renders the world the last Apply produced. Nothing here
	// decides anything: if the picture is wrong, the rules are wrong.
	Draw(screen *ebiten.Image)
	// Layout fixes the logical resolution, as Ebitengine means it.
	Layout(outsideWidth, outsideHeight int) (int, int)
}

// Options is everything the wrapper needs to run a game: the framework
// declaration, the rules, the play scene, and the engine's own options.
type Options[S, A, O any] struct {
	// Options is the framework declaration — name, accepted devices,
	// shared screen (api:run-wrapper).
	Options run.Options
	// Binding is the game's rules seam.
	Binding run.Binding[S, A, O]
	// Client is the play scene.
	Client Client[S, A, O]
	// Lobby configures the default ui:lobby-scene. A game that supplies
	// its own gathering screen sets Screen instead.
	Lobby LobbyOptions
	// Screen, when set, replaces the default lobby with the game's own
	// gathering screen. It is handed the roster of each new match and
	// must finalize it; see NewLobby for what the default one does.
	Screen func(*run.Roster[S, A, O]) Screen
	// Time is concept:game-time-control for the session clock. It is
	// independent of the frame rate: the tick loop runs on its own
	// goroutine, which is why a replay of this game reproduces exactly
	// and a headless build ticks the same way.
	Time session.TimeControl
	// Network, when set, is consulted once before the first lobby: the
	// preset either offers this instance's match or takes a seat on
	// somebody else's. Nil plays offline.
	//
	// A guest never sees the lobby. It has nothing to gather — the
	// roster it would fill belongs to the host.
	Matchmaking run.Matchmaking[S, A, O]
	// Record is where data:episode-log goes. A zero Root records
	// nothing; setting it is what turns ordinary play into a corpus.
	Record run.RecordOptions
	// Seed is the first match's seed; each later match in this process
	// adds its index, so a session of several matches reproduces from
	// one number (rule:shared-rng-seed).
	Seed uint64
	// Engine is the engine's own option struct, passed through to
	// RunGameWithOptions untouched.
	Engine *ebiten.RunGameOptions
	// WindowWidth and WindowHeight size the window. Zero leaves the
	// engine's default.
	WindowWidth, WindowHeight int
	// OnMatch, when set, is called when a match ends.
	OnMatch func(run.MatchResult)
}

// Run starts the game: the lobby first, then the play scene once a
// roster finalizes, then the lobby again when the match ends
// (concept:match-lifecycle).
//
// It gives the main goroutine to Ebitengine, which insists on it, and
// runs each session on its own.
func Run[S, A, O any](ctx context.Context, opts Options[S, A, O]) error {
	if err := opts.Options.Validate(); err != nil {
		return err
	}
	if err := opts.Binding.Validate(); err != nil {
		return err
	}
	if opts.Client == nil {
		return errors.New("eb: Options.Client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	a := &app[S, A, O]{opts: opts, ctx: ctx}
	if err := a.gather(); err != nil {
		return err
	}

	if opts.Options.Name != "" {
		ebiten.SetWindowTitle(opts.Options.Name)
	}
	if opts.WindowWidth > 0 && opts.WindowHeight > 0 {
		ebiten.SetWindowSize(opts.WindowWidth, opts.WindowHeight)
	}
	engine := opts.Engine
	if engine == nil {
		engine = &ebiten.RunGameOptions{}
	}
	err := ebiten.RunGameWithOptions(a, engine)
	a.stop()
	if errors.Is(err, ebiten.Termination) {
		return nil
	}
	return err
}

// app is the wrapper's ebiten.Game. It holds no rules: it owns the
// lifecycle and hands every frame to the current scene.
type app[S, A, O any] struct {
	opts Options[S, A, O]
	ctx  context.Context

	screen  Screen
	roster  *run.Roster[S, A, O]
	hosting run.Host[S, A, O]
	guest   run.Guest[S, A, O]
	match   *run.Match[S, A, O]
	rec     *run.Recording[S, A, O]
	watch   *run.Watcher[S, A, O]
	cancel  context.CancelFunc
	matches int
	last    *run.MatchResult
}

// Last reports the previous match's result, so a gathering scene can show
// how the last one went. Nil before the first match ends.
func (a *app[S, A, O]) Last() *run.MatchResult { return a.last }

// gather enters the gathering state: a fresh roster and the scene that
// fills it. A roster is per match, so this runs again after every match.
func (a *app[S, A, O]) gather() error {
	roster, err := run.NewRoster[S, A, O](a.opts.Options, a.opts.Binding.Slots)
	if err != nil {
		return err
	}
	a.roster = roster
	// A roster belongs to one match, so an offer still filling the
	// finished one would seat the next arrival nowhere.
	if a.hosting != nil {
		a.hosting.Rebind(roster)
	}
	if a.guest != nil {
		// The previous link is done with. Looking again is what lets
		// the same window rejoin, or host, without being restarted.
		_ = a.guest.Close()
		a.guest = nil
	}
	if a.opts.Screen != nil {
		a.screen = a.opts.Screen(roster)
		return nil
	}
	a.screen = NewLobby[S, A, O](a, roster)
	return nil
}

// Roster is the one being gathered.
func (a *app[S, A, O]) Roster() *run.Roster[S, A, O] { return a.roster }

// Network is the preset, or nil when this build plays offline.
func (a *app[S, A, O]) Matchmaking() run.Matchmaking[S, A, O] { return a.opts.Matchmaking }

// Context bounds the work a scene starts.
func (a *app[S, A, O]) Context() context.Context { return a.ctx }

// Declared hands a gathering scene the framework declaration.
func (a *app[S, A, O]) Declared() (run.Options, LobbyOptions, run.Binding[S, A, O]) {
	return a.opts.Options, a.opts.Lobby, a.opts.Binding
}

// Host is the offer this instance already makes, or nil.
func (a *app[S, A, O]) Host() run.Host[S, A, O] { return a.hosting }

// BecomeHost records that this instance is offering its match.
func (a *app[S, A, O]) BecomeHost(h run.Host[S, A, O]) { a.hosting = h }

// BecomeGuest records that this instance took a seat elsewhere, and
// switches to the scene that renders a link instead of a match.
func (a *app[S, A, O]) BecomeGuest(j run.Guest[S, A, O]) {
	a.guest = j
	a.screen = newRemote(a, j)
}

// Start finalizes the roster and moves to the play scene. A gathering
// scene calls it when it decides the roster is ready; the default lobby
// does so on a start press, and a game's own scene may do so on anything.
func (a *app[S, A, O]) Start() error {
	index := a.matches
	id := fmt.Sprintf("%s-%04d", a.opts.Options.Name, index)

	rec, err := run.OpenRecording[S, A, O](run.RecordOptions{
		Root:              a.opts.Record.Root,
		EpisodeID:         id,
		Mode:              a.opts.Record.Mode,
		ProtocolVersion:   pick(a.opts.Record.ProtocolVersion, a.opts.Binding.ProtocolVersion),
		EvaluationVersion: pickInt(a.opts.Record.EvaluationVersion, a.opts.Binding.EvaluationVersion),
		AgentKinds:        a.roster.AgentKinds(),
	})
	if err != nil {
		return err
	}

	cfg := a.opts.Binding.Config(id, a.opts.Seed+uint64(index))
	inner := cfg.Recorder
	if inner == nil {
		inner = rec.Recorder()
	}
	watch := run.Watch(inner)
	cfg.Recorder = watch
	a.watch = watch
	// The apply hook of api:tick-hooks: the committed world reaches the
	// renderer here, on the session's goroutine.
	appBroadcast := cfg.Broadcast
	client := a.opts.Client
	cfg.Broadcast = func(tick session.Tick, world *S) {
		if appBroadcast != nil {
			appBroadcast(tick, world)
		}
		client.Apply(tick, world)
	}

	if a.hosting != nil {
		a.hosting.Attach(&cfg)
	}

	match, err := a.roster.Finalize(cfg)
	if err != nil {
		rec.Close()
		return err
	}
	if a.hosting != nil {
		// Before the first tick: state produced with nobody to send it
		// to is state a guest would have to resync for.
		if err := a.hosting.Serve(a.ctx, match); err != nil {
			rec.Close()
			return err
		}
	}
	ctx, cancel := context.WithCancel(a.ctx)
	match.Start(ctx, a.opts.Time)

	a.match, a.rec, a.cancel = match, rec, cancel
	a.screen = &play[S, A, O]{app: a, client: client, match: match}
	return nil
}

// ended closes out a finished match and returns to gathering.
func (a *app[S, A, O]) ended() error {
	res := run.MatchResult{
		Index: a.matches,
		Seed:  a.opts.Seed + uint64(a.matches),
		Ticks: a.match.Tick(),
		Err:   a.match.Err(),
		Seats: a.match.Seats(),
	}
	if a.watch != nil {
		res.Outcomes = a.watch.Outcomes()
	}
	if a.rec != nil {
		res.EpisodeDir = a.rec.Dir
	}
	a.stop()
	a.matches++
	a.last = &res
	if a.opts.OnMatch != nil {
		a.opts.OnMatch(res)
	}
	// The board stays up until somebody dismisses it. Going straight
	// back to the lobby would take the last frame away before they had
	// seen how it ended.
	a.screen = newResult(a, outcomeLine(&res))
	return nil
}

// stop releases the running match, if any.
func (a *app[S, A, O]) stop() {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if a.rec != nil {
		a.rec.Close()
		a.rec = nil
	}
	a.match = nil
}

// Update gives the frame to the current scene.
func (a *app[S, A, O]) Update() error { return a.screen.Update() }

// Draw gives the screen to the current scene.
func (a *app[S, A, O]) Draw(screen *ebiten.Image) { a.screen.Draw(screen) }

// Layout uses the game's logical resolution for every scene, lobby
// included, so a game has one coordinate system rather than two.
func (a *app[S, A, O]) Layout(w, h int) (int, int) { return a.opts.Client.Layout(w, h) }

// play is the scene of a running match. It is thin on purpose: intake,
// then let the session tick on its own clock.
type play[S, A, O any] struct {
	app    *app[S, A, O]
	client Client[S, A, O]
	match  *run.Match[S, A, O]
}

// Update runs the intake hook, then checks whether the match is over.
func (p *play[S, A, O]) Update() error {
	if p.match.Over() {
		return p.app.ended()
	}
	p.client.Intake(p.match)
	return nil
}

// Draw renders the world the client retained.
func (p *play[S, A, O]) Draw(screen *ebiten.Image) { p.client.Draw(screen) }

func pick(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func pickInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
