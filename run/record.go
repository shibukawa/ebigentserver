package run

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/session"
)

// EpisodeID names one match's episode directory: the game name and the
// match index, zero-padded so a corpus lists in the order it was played.
func EpisodeID(name string, index int) string {
	return fmt.Sprintf("%s-%04d", name, index)
}

// ResumeIndex reports the first match index at or after from that no
// episode directory under root is already using.
//
// It exists because a match index is not only a name. It also carries
// the seed (rule:shared-rng-seed), so a second launch that started
// counting at zero again would both write over the episodes the first
// one recorded and replay its seeds — and a corpus of the same match
// recorded twice is worth one match, however many files it holds.
//
// The engine wrapper calls this once before its first lobby, because a
// person who plays, quits, and comes back is adding to one corpus. Serve
// does not: it is given a match count and a seed up front, so a headless
// batch is a reproducible unit and re-running it is meant to produce the
// same episodes. A headless caller that wants to accumulate instead
// passes a resumed index in as its own starting seed.
//
// An empty root, or a root that cannot be read, returns from unchanged:
// recording degrades rather than interrupting play.
func ResumeIndex(root, name string, from int) int {
	if root == "" {
		return from
	}
	for {
		if _, err := os.Stat(filepath.Join(root, EpisodeID(name, from))); err != nil {
			return from
		}
		from++
	}
}

// Recording is one episode's data:episode-log on disk, opened under a
// corpus root as root/<id>/ so a run of many matches accumulates a corpus
// the analysis and distillation tools read directly.
//
// Recording is why a solo game is worth putting on this framework at all:
// every decision every controller made — the player's and each enemy's —
// lands here with the sight it was made from, which is the input
// flow:behavior-tree-synthesis needs and the one thing a hand-written
// enemy loop never produces.
type Recording[W, A, S any] struct {
	// Writer implements session.Recorder; hand it to session.Config.
	Writer *episode.Writer[W, A, S]
	// Dir is the episode directory that was created.
	Dir string

	files []*os.File
}

// RecordOptions is what a run declares about recording. It mirrors the
// episode section of data:run-config, so a caller can pass config values
// through unchanged.
type RecordOptions struct {
	// Root is the corpus directory. Empty records nothing at zero cost.
	Root string
	// EpisodeID names the subdirectory under Root. Empty uses the
	// session id.
	EpisodeID string
	// Mode is concept:episode-recording-mode. Empty means
	// analysis_sampled, matching the run config default: a corpus is
	// the common case and a bit-exact replay log is the deliberate one.
	Mode episode.Mode
	// ProtocolVersion is data:protocol-version, recorded so a corpus
	// cannot be mixed across incompatible schemas.
	ProtocolVersion string
	// EvaluationVersion versions the game's evaluation function, since
	// changing it invalidates comparisons across a corpus.
	EvaluationVersion int
	// AgentKinds labels slots; Roster.AgentKinds supplies it.
	AgentKinds map[session.SlotID]string
}

// OpenRecording creates the episode directory and its streams. A zero
// Root returns a nil recording and no error, so a caller can wire it
// unconditionally and let configuration decide.
func OpenRecording[W, A, S any](opts RecordOptions) (*Recording[W, A, S], error) {
	if opts.Root == "" {
		return nil, nil
	}
	id := opts.EpisodeID
	if id == "" {
		return nil, fmt.Errorf("run: recording under %s needs an episode id", opts.Root)
	}
	mode := opts.Mode
	if mode != episode.ReplayComplete && mode != episode.AnalysisSampled {
		mode = episode.AnalysisSampled
	}
	dir := filepath.Join(opts.Root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("run: create episode directory: %w", err)
	}

	rec := &Recording[W, A, S]{Dir: dir}
	open := func(name string) (*os.File, error) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		rec.files = append(rec.files, f)
		return f, nil
	}
	var streams episode.Streams
	var err error
	if streams.Decisions, err = open("decisions.jsonl"); err != nil {
		rec.Close()
		return nil, err
	}
	if streams.Events, err = open("events.jsonl"); err != nil {
		rec.Close()
		return nil, err
	}
	if streams.Outcomes, err = open("outcomes.jsonl"); err != nil {
		rec.Close()
		return nil, err
	}
	// The world stream is the ground truth a bit-identical replay needs;
	// a sampled corpus drops it, and NewWriter enforces that regardless
	// of what is passed here.
	if mode == episode.ReplayComplete {
		if streams.World, err = open("world.jsonl"); err != nil {
			rec.Close()
			return nil, err
		}
	}

	rec.Writer = episode.NewWriter[W, A, S](streams, mode, episode.Meta{
		EpisodeID:         id,
		ProtocolVersion:   opts.ProtocolVersion,
		EvaluationVersion: opts.EvaluationVersion,
		AgentKinds:        opts.AgentKinds,
	})
	return rec, nil
}

// Close flushes and closes every stream. It reports the writer's first
// write error if there was one: recording degrades rather than
// interrupting play, so the failure surfaces here instead.
func (r *Recording[W, A, S]) Close() error {
	if r == nil {
		return nil
	}
	for _, f := range r.files {
		f.Close()
	}
	r.files = nil
	if r.Writer != nil {
		return r.Writer.Err()
	}
	return nil
}

// Recorder returns the session recorder, or nil for a nil recording, so
// session.Config.Recorder can be assigned unconditionally.
func (r *Recording[W, A, S]) Recorder() session.Recorder[W, A, S] {
	if r == nil || r.Writer == nil {
		return nil
	}
	return r.Writer
}
