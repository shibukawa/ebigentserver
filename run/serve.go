package run

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/session"
)

// Binding is how a game hands its rules to the wrapper. Both halves take
// it: the engine half to play, this one to serve or simulate.
//
// It is small on purpose. The wrapper needs to know which seats exist,
// how to build a session for one match, and how to fill a seat with an
// agent. Everything else — physics, projection, validation, evaluation —
// is already inside the session config the game supplies.
type Binding[W, A, S any] struct {
	// Slots is the game's declared slot set (concept:player-slot).
	Slots []session.SlotID
	// Config builds the session configuration for one match. A fresh
	// seed per match is what keeps a corpus from collapsing into
	// duplicates (rule:shared-rng-seed).
	Config func(id string, seed uint64) session.Config[W, A, S]
	// NewAgent supplies the controller for a bot seat: the enemies of a
	// solo game, the opponent of a practice match, the stand-in for a
	// seat nobody took. The returned id labels the seat in the lobby
	// and the episode header — an enemy kind belongs here, because that
	// is what makes a corpus separable per kind later.
	//
	// Optional. A seat left empty is not necessarily a seat for a bot:
	// a person arriving over a link is an ordinary concept:agent too,
	// and one that cannot exist before the game is running. A game that
	// only waits for people therefore has no factory to name, and
	// FillBots — the one caller — reports its absence where it matters.
	NewAgent func(slot session.SlotID) (id string, agent session.Agent[S, A])
	// Agents names the controllers a run may ask for by name, which
	// NewAgent alone cannot express: it answers "who sits in this seat
	// if nobody said", and that is a different question from "seat the
	// chaser here".
	//
	// Optional, and a game with one kind of bot needs none of it. It
	// earns its place as soon as a game has several: a corpus mixing
	// three pursuit styles distills into a policy none of them had, so
	// recording one kind at a time is the difference between a usable
	// corpus and a wasted one — and which kind is a property of the run
	// rather than of the rules.
	//
	// The names are the ids NewAgent would return, so a seat filled by
	// either route lands in data:episode-log under the same label.
	//
	// Each constructor is handed the match's seed, because a training
	// run needs an opponent that varies and an agent has no other way
	// to get randomness it is allowed to have: rule:shared-rng-seed
	// puts the one seed a match may derive from in the session, and
	// this is where a controller is built. A deterministic agent
	// ignores it, and four hundred matches against one that does are
	// one match written down four hundred times.
	Agents map[string]func(seed uint64) session.Agent[S, A]
	// ProtocolVersion and EvaluationVersion travel into every episode
	// header so a corpus cannot silently mix incompatible runs.
	ProtocolVersion   string
	EvaluationVersion int
}

// Validate rejects a binding the wrapper cannot use.
func (b Binding[W, A, S]) Validate() error {
	if len(b.Slots) == 0 {
		return errors.New("run: Binding.Slots is required")
	}
	if b.Config == nil {
		return errors.New("run: Binding.Config is required")
	}
	for name, build := range b.Agents {
		if name == "" {
			return errors.New("run: Binding.Agents has an unnamed entry; the name is what a run asks for and what labels the seat in an episode")
		}
		if build == nil {
			return fmt.Errorf("run: Binding.Agents[%q] is nil", name)
		}
	}
	return nil
}

// AgentKinds are the names this binding can be asked for, sorted so a
// report and an error message list them the same way twice.
func (b Binding[W, A, S]) AgentKinds() []string {
	names := make([]string, 0, len(b.Agents))
	for name := range b.Agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ServeOptions declares one headless run: what to play, how many times,
// and where the log goes.
type ServeOptions struct {
	// Agents assigns a named controller to a seat, seeding api:roster
	// the way the slot table of data:run-config does. A slot named here
	// is filled from Binding.Agents; every other bot seat still comes
	// from NewAgent, so naming one seat does not mean naming them all.
	Agents map[session.SlotID]string
	// Matches is how many matches to play. 0 plays until ctx is
	// cancelled, which is what a dedicated server does.
	Matches int
	// Seed is the first match's seed; each later match adds its index,
	// so the whole run reproduces from this one number.
	Seed uint64
	// Time is concept:game-time-control. Paced is real time, which a
	// server wants; Unlimited runs as fast as the machine allows, which
	// is what makes a training run finish in seconds.
	Time session.TimeControl
	// Record is where episodes go. A zero Root records nothing.
	Record RecordOptions
	// OnMatch, when set, is called after each match with its result.
	OnMatch func(MatchResult)
}

// MatchResult is what one headless match produced.
type MatchResult struct {
	// Index counts matches from zero.
	Index int
	// Seed is the seed this match ran with.
	Seed uint64
	// Ticks is how many ticks committed.
	Ticks session.Tick
	// EpisodeDir is where the log was written, empty when not
	// recording.
	EpisodeDir string
	// Err is why the match ended abnormally, nil after a normal end.
	Err error
	// Seats is the roster that played it.
	Seats []Seat
	// Outcomes is every seat's final data:evaluation-signal, so a
	// training loop can report win rates without re-reading the episode
	// it just wrote.
	Outcomes []session.SlotOutcome
}

// Serve runs the match loop of concept:match-lifecycle with no screen:
// gather, run, report, gather again. Every seat is filled by
// Binding.NewAgent, so the same rules that carry a human at a keyboard
// produce a corpus with nobody watching.
//
// This is the headless half of an AI development cycle: run it to
// generate data:episode-log, distill the recorded decisions into
// data:behavior-chip, regenerate the agent, run it again against the
// previous one.
//
// A dedicated server is the same loop with the gathering step fed by a
// listener instead of by NewAgent. That wiring is the transport seam and
// is not connected here yet.
func Serve[W, A, S any](ctx context.Context, opts Options, b Binding[W, A, S], sp ServeOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	if err := b.Validate(); err != nil {
		return err
	}
	for i := 0; sp.Matches == 0 || i < sp.Matches; i++ {
		if ctx.Err() != nil {
			return nil
		}
		res, err := serveOne(ctx, opts, b, sp, i)
		if err != nil {
			return err
		}
		if sp.OnMatch != nil {
			sp.OnMatch(res)
		}
		if res.Err != nil && !errors.Is(res.Err, context.Canceled) {
			return res.Err
		}
	}
	return nil
}

// serveOne gathers, runs, and closes one match.
func serveOne[W, A, S any](ctx context.Context, opts Options, b Binding[W, A, S], sp ServeOptions, index int) (MatchResult, error) {
	seed := sp.Seed + uint64(index)
	id := EpisodeID(opts.Name, index)

	roster, err := NewRoster[W, A, S](opts, b.Slots)
	if err != nil {
		return MatchResult{}, err
	}
	if err := roster.FillNamed(b.Agents, sp.Agents, seed); err != nil {
		return MatchResult{}, err
	}
	if err := roster.FillBots(b.NewAgent); err != nil {
		return MatchResult{}, err
	}

	rec, err := OpenRecording[W, A, S](RecordOptions{
		Root:              sp.Record.Root,
		EpisodeID:         id,
		Mode:              recordMode(sp.Record.Mode),
		ProtocolVersion:   pick(sp.Record.ProtocolVersion, b.ProtocolVersion),
		EvaluationVersion: pickInt(sp.Record.EvaluationVersion, b.EvaluationVersion),
		AgentKinds:        roster.AgentKinds(),
	})
	if err != nil {
		return MatchResult{}, err
	}

	cfg := b.Config(id, seed)
	inner := cfg.Recorder
	if inner == nil {
		inner = rec.Recorder()
	}
	watch := Watch(inner)
	cfg.Recorder = watch

	match, err := roster.Finalize(cfg)
	if err != nil {
		rec.Close()
		return MatchResult{}, err
	}

	runErr := match.Run(ctx, sp.Time)
	res := MatchResult{
		Index:    index,
		Seed:     seed,
		Ticks:    match.Tick(),
		Err:      runErr,
		Seats:    match.Seats(),
		Outcomes: watch.Outcomes(),
	}
	if rec != nil {
		res.EpisodeDir = rec.Dir
	}
	if err := rec.Close(); err != nil {
		return res, fmt.Errorf("run: episode %s: %w", id, err)
	}
	return res, nil
}

// recordMode resolves the zero mode.
func recordMode(m episode.Mode) episode.Mode {
	if m == "" {
		return episode.AnalysisSampled
	}
	return m
}

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
