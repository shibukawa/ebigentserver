package pong_test

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/pong/msg"
	"github.com/shibukawa/ebigentserver/samples/pong/pong"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
)

var testTuning = session.TuningProfile{TickRate: 60, SendRate: 20, HistoryDepth: 8, SnapshotEvery: 30}

// patternInput is a deterministic input schedule: paddles wobble on fixed
// cycles, so points fall and the game ends without any agent involved.
func patternInput(tick session.Tick, slot session.SlotID) (pong.Input, bool) {
	if tick%2 == 1 { // half the ticks are silent
		return pong.Input{}, false
	}
	phase := tick / 2
	var move int8
	if slot == pong.SlotLeft {
		move = []int8{1, 1, 0, -1, -1, -1, 0, 1}[phase%8]
	} else {
		move = []int8{-1, 0, 1, 1, 0, -1, -1, 0}[phase%8]
	}
	return pong.Input{Tick: uint32(tick), MoveY: move}, true
}

func newSession(t *testing.T, cfg func(*session.Config[pong.State, pong.Input, pong.Observation])) *session.Session[pong.State, pong.Input, pong.Observation] {
	t.Helper()
	c := session.Config[pong.State, pong.Input, pong.Observation]{
		ID:        "pong-test",
		Slots:     pong.Slots(),
		RuleSet:   pong.RuleSet{},
		Validator: pong.Validator{},
		Canonical: pong.Canonical,
		Tuning:    &testTuning,
		Clock:     func() int64 { return 0 },
	}
	if cfg != nil {
		cfg(&c)
	}
	s, err := session.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	for _, slot := range pong.Slots() {
		if err := s.Admit(slot, session.Detached[pong.Observation, pong.Input]{}); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

type logs struct {
	decisions, events, outcomes, world bytes.Buffer
}

func record(t *testing.T, src func(session.Tick, session.SlotID) (pong.Input, bool)) (*logs, session.Tick) {
	t.Helper()
	var l logs
	w := episode.NewWriter[pong.State, pong.Input, pong.Observation](
		episode.Streams{Decisions: &l.decisions, Events: &l.events, Outcomes: &l.outcomes, World: &l.world},
		episode.ReplayComplete,
		episode.Meta{EpisodeID: "pong-ep", ProtocolVersion: msg.CBORProtocolVersion},
	)
	s := newSession(t, func(c *session.Config[pong.State, pong.Input, pong.Observation]) {
		c.Seed = 7
		c.Recorder = w
		c.InputSource = src
	})
	if err := s.RunRealtime(context.Background(), session.Unlimited); err != nil {
		t.Fatal(err)
	}
	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	if err := w.Err(); err != nil {
		t.Fatalf("recording failed: %v", err)
	}
	return &l, s.Tick()
}

// Phase 2's determinism criteria carried into realtime: the scripted
// match ends identically every run, and its final checkpoint is pinned so
// darwin/arm64 and CI's linux/amd64 must agree bit for bit.
func TestScriptedMatchDigestPinned(t *testing.T) {
	l, ticks := record(t, patternInput)
	cps, err := episode.ReadCheckpoints(bytes.NewReader(l.events.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) == 0 {
		t.Fatal("no checkpoints recorded")
	}
	last := cps[len(cps)-1]
	const wantTick, wantWorld, wantAction = 585, "3fa3d99db6a58444", "a221fc1c8243618d"
	if session.Tick(last.Tick) != wantTick || last.WorldHash != wantWorld || last.ActionHash != wantAction {
		t.Fatalf("final checkpoint tick %d (of %d ticks): world %s action %s (pinned tick %d / %s / %s)",
			last.Tick, ticks, last.WorldHash, last.ActionHash, wantTick, wantWorld, wantAction)
	}
}

// A recorded realtime match replays bit-identically from the log alone:
// the schedule reader re-feeds each accepted input at its recorded tick.
func TestRealtimeRecordReplaysBitIdentical(t *testing.T) {
	original, _ := record(t, patternInput)
	_, schedule, err := episode.ReadReplaySchedule[pong.Input](bytes.NewReader(original.decisions.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	replayed, _ := record(t, schedule)
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
}

// The full loopback stack: session tick loop, hub fan-out, snapshot/delta
// reconstruction, bots deciding off the reconstructed state, inputs
// flowing back through inboxes. Paced realtime, stopped by the context;
// the assertion is that the pipeline moved real state, not who won.
func TestLoopbackBotMatch(t *testing.T) {
	hub, err := statesync.NewHub(pong.Codec(), testTuning)
	if err != nil {
		t.Fatal(err)
	}
	fast := testTuning
	fast.TickRate, fast.SendRate = 240, 240 // tight loop so the test finishes quickly
	s := newSession(t, func(c *session.Config[pong.State, pong.Input, pong.Observation]) {
		c.Tuning = &fast
		c.Broadcast = hub.Broadcast
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	var wg sync.WaitGroup
	for _, slot := range pong.Slots() {
		down, err := hub.Attach(slot)
		if err != nil {
			t.Fatal(err)
		}
		inbox, err := s.Inbox(slot)
		if err != nil {
			t.Fatal(err)
		}
		client := &pong.Client{Slot: slot, Agent: &pong.Bot{}, Inbox: inbox, Hub: hub, Down: down, Tuning: fast}
		wg.Add(1)
		go client.Run(ctx, &wg)
	}

	if err := s.RunRealtime(ctx, session.Paced); err != nil {
		t.Fatal(err)
	}
	hub.Close()
	cancel()
	wg.Wait()

	if s.State() != session.StateEnded {
		t.Fatalf("state = %v, want ended", s.State())
	}
	if s.Tick() < 60 {
		t.Fatalf("only %d ticks committed in 1.5s at 240Hz", s.Tick())
	}
}
