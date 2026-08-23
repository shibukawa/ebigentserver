package tron_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/tron/msg"
	"github.com/shibukawa/ebigentserver/samples/tron/tron"
	"github.com/shibukawa/ebigentserver/session"
)

func slots(n int) []session.SlotID {
	out := make([]session.SlotID, n)
	for i := range out {
		out[i] = session.SlotID(i + 1)
	}
	return out
}

var testTuning = session.TuningProfile{
	TickRate: 120, SendRate: 60, HistoryDepth: 16, SnapshotEvery: 0,
	BaselineMode: session.BaselineBounded, SpeculationDepth: 8,
	AckMode: 2, RejectionThreshold: 8, SilenceDeadline: 240,
}

func TestRules(t *testing.T) {
	g := tron.RuleSet{SlotIDs: slots(2)}
	s := g.Start(0)
	if s.Alive != 2 || len(s.Players) != 2 {
		t.Fatalf("start: %+v", s)
	}
	v := tron.Validator{}
	p0 := s.Players[0] // spawned facing down
	if err := v.Legal(&s, session.SlotID(p0.ID), tron.Input{Dir: tron.DirUp}); err == nil {
		t.Error("reversal must be illegal")
	}
	if err := v.Legal(&s, session.SlotID(p0.ID), tron.Input{Dir: 9}); err == nil {
		t.Error("direction out of range must be illegal")
	}
	if err := v.Legal(&s, session.SlotID(p0.ID), tron.Input{Dir: tron.DirLeft}); err != nil {
		t.Errorf("legal turn rejected: %v", err)
	}

	// Driving into the wall kills; the survivor wins.
	g.Apply(&s, session.SlotID(s.Players[0].ID), tron.Input{Dir: tron.DirLeft})
	for range msg.GridW {
		g.Advance(&s)
		if s.Over {
			break
		}
	}
	if !s.Over || s.Winner != s.Players[1].ID {
		t.Fatalf("expected player 2 to win, state %+v", s)
	}
	sig := g.Evaluate(&s, session.SlotID(s.Players[1].ID))
	if sig.Terminal != session.Win {
		t.Fatalf("winner signal = %v", sig.Terminal)
	}
	if g.Evaluate(&s, session.SlotID(s.Players[0].ID)).Terminal != session.Lose {
		t.Fatal("loser signal not Lose")
	}
}

func TestTrailGrowsAndBlocks(t *testing.T) {
	g := tron.RuleSet{SlotIDs: slots(2)}
	s := g.Start(0)
	for range 5 {
		g.Advance(&s)
	}
	if len(s.Trail) != 10 { // 2 players × 5 ticks
		t.Fatalf("trail length = %d, want 10", len(s.Trail))
	}
	// Trail ids are the append-only identity sequence the delta rides on.
	for i, c := range s.Trail {
		if c.ID != uint32(i) {
			t.Fatalf("trail id %d at index %d", c.ID, i)
		}
	}
}

func TestPlausibilityRejectsFutureInputs(t *testing.T) {
	suspects := 0
	tuning := testTuning
	tuning.RejectionThreshold = 1
	s, err := session.New(session.Config[tron.State, tron.Input, tron.Observation]{
		ID:           "tron-plausibility",
		Slots:        slots(2),
		RuleSet:      tron.RuleSet{SlotIDs: slots(2)},
		Validator:    tron.Validator{},
		Plausibility: tron.Plausibility{FutureWindow: 120},
		Canonical:    tron.Canonical,
		Tuning:       &tuning,
		Clock:        func() int64 { return 0 },
		OnSuspect:    func(session.SlotID, int32) { suspects++ },
		InputSource: func(tick session.Tick, slot session.SlotID) (tron.Input, bool) {
			if tick == 0 && slot == 1 {
				return tron.Input{Tick: 99999, Dir: tron.DirLeft}, true // stamped absurdly ahead
			}
			return tron.Input{}, false
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	for _, slot := range slots(2) {
		if err := s.Admit(slot, session.Detached[tron.Observation, tron.Input]{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RunRealtime(context.Background(), session.Unlimited); err != nil {
		t.Fatal(err)
	}
	if suspects != 1 {
		t.Fatalf("OnSuspect fired %d times, want 1", suspects)
	}
}

// patternInput steers 8 cycles deterministically so the match ends by
// collisions without any agent.
func patternInput(tick session.Tick, slot session.SlotID) (tron.Input, bool) {
	if uint64(tick)%16 != uint64(slot)%16 {
		return tron.Input{}, false
	}
	return tron.Input{Tick: uint32(tick), Dir: uint8((uint64(slot) + uint64(tick)/16) % 4)}, true
}

// The 8-player scripted match is deterministic and its final checkpoint
// is pinned across architectures, extending the phase 2/3 chain.
func TestEightPlayerDigestPinned(t *testing.T) {
	run := func() (session.Tick, episode.Event) {
		var events bytes.Buffer
		w := episode.NewWriter[tron.State, tron.Input, tron.Observation](
			episode.Streams{Events: &events},
			episode.ReplayComplete,
			episode.Meta{EpisodeID: "tron-8", ProtocolVersion: msg.SchemaVersion},
		)
		tuning := testTuning
		s, err := session.New(session.Config[tron.State, tron.Input, tron.Observation]{
			ID: "tron-8", Slots: slots(8),
			RuleSet:   tron.RuleSet{SlotIDs: slots(8)},
			Validator: tron.Validator{}, Canonical: tron.Canonical,
			Tuning: &tuning, Clock: func() int64 { return 0 },
			Recorder: w, Seed: 21, InputSource: patternInput,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.OpenAdmission(); err != nil {
			t.Fatal(err)
		}
		for _, slot := range slots(8) {
			if err := s.Admit(slot, session.Detached[tron.Observation, tron.Input]{}); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.RunRealtime(context.Background(), session.Unlimited); err != nil {
			t.Fatal(err)
		}
		cps, err := episode.ReadCheckpoints(bytes.NewReader(events.Bytes()))
		if err != nil || len(cps) == 0 {
			t.Fatalf("checkpoints: %v (%d)", err, len(cps))
		}
		return s.Tick(), cps[len(cps)-1]
	}
	ticks, last := run()
	ticks2, last2 := run()
	if ticks != ticks2 || last != last2 {
		t.Fatalf("two runs diverged: %d/%+v vs %d/%+v", ticks, last, ticks2, last2)
	}
	// The world digests moved once, when concept:cbor-world-profile became the
	// map shape of tinybind v0.5.23: the profile's integer labels are gone and
	// members carry their names. The action digests did not move, because the
	// array shape encodes byte for byte what the wire profile did — which is
	// what shows the encoding changed and the simulation did not.
	const wantTick, wantWorld, wantAction = 33, "f3e117369e1acf88", "a2c67407a65952f1"
	if last.Tick != wantTick || last.WorldHash != wantWorld || last.ActionHash != wantAction {
		t.Fatalf("final checkpoint tick %d world %s action %s (pinned %d / %s / %s)",
			last.Tick, last.WorldHash, last.ActionHash, wantTick, wantWorld, wantAction)
	}
}
