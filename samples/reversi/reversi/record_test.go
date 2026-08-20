package reversi_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/reversi/reversi"
	"github.com/shibukawa/ebigentserver/session"
)

type logs struct {
	decisions, events, outcomes, world bytes.Buffer
}

// recordMatch plays one full match with the given agents, recording it in
// the given mode, and returns the four streams.
func recordMatch(t *testing.T, black, white session.Agent[reversi.Observation, reversi.Move], mode episode.Mode, kinds map[session.SlotID]string) *logs {
	t.Helper()
	var l logs
	w := episode.NewWriter[reversi.State, reversi.Move, reversi.Observation](
		episode.Streams{Decisions: &l.decisions, Events: &l.events, Outcomes: &l.outcomes, World: &l.world},
		mode,
		episode.Meta{EpisodeID: "reversi-ep-1", AgentKinds: kinds},
	)
	s := newMatch(t, black, white, func(c *session.Config[reversi.State, reversi.Move, reversi.Observation]) {
		c.Seed = 42
		c.Recorder = w
	})
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	if err := w.Err(); err != nil {
		t.Fatalf("recording failed: %v", err)
	}
	return &l
}

// Phase 2 completion criterion: a recorded match replays bit-identically.
// The replay agents are seated from the log alone; every stream the
// replay produces — decisions, checkpoints, outcomes — must equal the
// original byte for byte.
func TestRecordedMatchReplaysBitIdentical(t *testing.T) {
	original := recordMatch(t, &reversi.GreedyBot{}, &reversi.FirstBot{}, episode.ReplayComplete, nil)

	rep, err := episode.ReadReplay[reversi.Move](bytes.NewReader(original.decisions.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Header.Seed != 42 {
		t.Fatalf("header seed = %d, want 42", rep.Header.Seed)
	}
	replayed := recordMatch(t,
		&session.ReplayAgent[reversi.Observation, reversi.Move]{Actions: rep.Actions[reversi.SlotBlack]},
		&session.ReplayAgent[reversi.Observation, reversi.Move]{Actions: rep.Actions[reversi.SlotWhite]},
		episode.ReplayComplete, nil)

	for _, cmp := range []struct {
		name          string
		first, second *bytes.Buffer
	}{
		{"decisions", &original.decisions, &replayed.decisions},
		{"events", &original.events, &replayed.events},
		{"outcomes", &original.outcomes, &replayed.outcomes},
		{"world", &original.world, &replayed.world},
	} {
		if !bytes.Equal(cmp.first.Bytes(), cmp.second.Bytes()) {
			t.Errorf("%s stream differs between original and replay", cmp.name)
		}
	}

	// The checkpoint chain is the determinism proof; make sure it exists
	// rather than trivially matching as empty.
	cps, err := episode.ReadCheckpoints(bytes.NewReader(original.events.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) < 40 {
		t.Fatalf("only %d checkpoints recorded", len(cps))
	}
}

// Phase 2 acceptance: the same episode digested on arm64 and amd64 must
// match. This pins the final data:state-checkpoint of the canonical
// greedy-vs-first match; the CI matrix runs it on amd64 while development
// runs it on darwin/arm64, so a divergence fails one of them.
func TestFinalCheckpointIsPinnedAcrossArchitectures(t *testing.T) {
	l := recordMatch(t, &reversi.GreedyBot{}, &reversi.FirstBot{}, episode.ReplayComplete, nil)
	cps, err := episode.ReadCheckpoints(bytes.NewReader(l.events.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	last := cps[len(cps)-1]
	const wantWorld, wantAction = "22be68a0f2609778", "3531fdab7080cdbe"
	if last.WorldHash != wantWorld || last.ActionHash != wantAction {
		t.Fatalf("final checkpoint tick %d: world %s action %s (pinned %s / %s) — a protocol-relevant bit moved",
			last.Tick, last.WorldHash, last.ActionHash, wantWorld, wantAction)
	}
}

// concept:episode-recording-mode: a sampled log never masquerades as
// replayable — the reader refuses it and the writer drops the world
// stream and checkpoints even when destinations are supplied.
func TestSampledLogIsNotReplayable(t *testing.T) {
	l := recordMatch(t, &reversi.GreedyBot{}, &reversi.FirstBot{}, episode.AnalysisSampled, nil)
	if _, err := episode.ReadReplay[reversi.Move](bytes.NewReader(l.decisions.Bytes())); !errors.Is(err, episode.ErrNotReplayable) {
		t.Fatalf("ReadReplay error = %v, want ErrNotReplayable", err)
	}
	if l.world.Len() != 0 {
		t.Error("sampled log recorded world ground truth")
	}
	cps, err := episode.ReadCheckpoints(bytes.NewReader(l.events.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) != 0 {
		t.Errorf("sampled log recorded %d checkpoints", len(cps))
	}
}

// The outcomes stream carries metric:episode-outcome for every slot.
func TestOutcomesStream(t *testing.T) {
	l := recordMatch(t, &reversi.GreedyBot{}, &reversi.FirstBot{}, episode.ReplayComplete,
		map[session.SlotID]string{reversi.SlotBlack: "greedy-bot", reversi.SlotWhite: "first-bot"})
	lines := bytes.Split(bytes.TrimSpace(l.outcomes.Bytes()), []byte("\n"))
	if len(lines) != 3 { // header + 2 slots
		t.Fatalf("outcomes stream has %d lines, want 3", len(lines))
	}
	for _, want := range []string{`"stream":"outcomes"`, `"episode_id":"reversi-ep-1"`} {
		if !bytes.Contains(lines[0], []byte(want)) {
			t.Errorf("outcomes header missing %s: %s", want, lines[0])
		}
	}
	// One slot won or both drew; no row may be non-terminal.
	for _, row := range lines[1:] {
		if bytes.Contains(row, []byte(`"result":"not_terminal"`)) {
			t.Errorf("non-terminal outcome row: %s", row)
		}
	}
	// agent_kind labels reached the decisions stream.
	if !bytes.Contains(l.decisions.Bytes(), []byte(`"agent_kind":"greedy-bot"`)) {
		t.Error("decisions stream missing agent_kind label")
	}
}
