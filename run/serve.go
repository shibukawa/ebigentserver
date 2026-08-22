package run

import (
	"context"
	"errors"
	"fmt"

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
type Binding[S, A, O any] struct {
	// Slots is the game's declared slot set (concept:player-slot).
	Slots []session.SlotID
	// Config builds the session configuration for one match. A fresh
	// seed per match is what keeps a corpus from collapsing into
	// duplicates (rule:shared-rng-seed).
	Config func(id string, seed uint64) session.Config[S, A, O]
	// NewAgent supplies the controller for a bot seat: the enemies of a
	// solo game, the opponent of a practice match, the stand-in for a
	// seat nobody took. The returned id labels the seat in the lobby
	// and the episode header — an enemy kind belongs here, because that
	// is what makes a corpus separable per kind later.
	NewAgent func(slot session.SlotID) (id string, agent session.Agent[O, A])
	// ProtocolVersion and EvaluationVersion travel into every episode
	// header so a corpus cannot silently mix incompatible runs.
	ProtocolVersion   string
	EvaluationVersion int
}

// Validate rejects a binding the wrapper cannot use.
func (b Binding[S, A, O]) Validate() error {
	if len(b.Slots) == 0 {
		return errors.New("run: Binding.Slots is required")
	}
	if b.Config == nil {
		return errors.New("run: Binding.Config is required")
	}
	if b.NewAgent == nil {
		return errors.New("run: Binding.NewAgent is required; a seat nobody takes still needs a controller")
	}
	return nil
}

// ServeOptions declares one headless run: what to play, how many times,
// and where the log goes.
type ServeOptions struct {
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
func Serve[S, A, O any](ctx context.Context, opts Options, b Binding[S, A, O], sp ServeOptions) error {
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
func serveOne[S, A, O any](ctx context.Context, opts Options, b Binding[S, A, O], sp ServeOptions, index int) (MatchResult, error) {
	seed := sp.Seed + uint64(index)
	id := fmt.Sprintf("%s-%04d", opts.Name, index)

	roster, err := NewRoster[S, A, O](opts, b.Slots)
	if err != nil {
		return MatchResult{}, err
	}
	if err := roster.FillBots(b.NewAgent); err != nil {
		return MatchResult{}, err
	}

	rec, err := OpenRecording[S, A, O](RecordOptions{
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
