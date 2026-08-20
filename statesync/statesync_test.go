package statesync_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
)

// A minimal hand-rolled codec: enough structure to prove the sender and
// receiver logic without dragging generated code into framework tests.
type world struct {
	A, B int64
}

type delta struct {
	A, B *int64
}

func codec() statesync.Codec[world, delta] {
	return statesync.Codec[world, delta]{
		AppendSnapshot: func(dst []byte, s *world) []byte { b, _ := json.Marshal(s); return append(dst, b...) },
		DecodeSnapshot: func(s *world, data []byte) error { return json.Unmarshal(data, s) },
		Diff: func(base, cur *world) delta {
			var d delta
			if base.A != cur.A {
				v := cur.A
				d.A = &v
			}
			if base.B != cur.B {
				v := cur.B
				d.B = &v
			}
			return d
		},
		AppendDelta: func(dst []byte, d *delta) []byte { b, _ := json.Marshal(d); return append(dst, b...) },
		DecodeDelta: func(d *delta, data []byte) error { return json.Unmarshal(data, d) },
		ApplyDelta: func(s *world, d delta) error {
			if d.A != nil {
				s.A = *d.A
			}
			if d.B != nil {
				s.B = *d.B
			}
			return nil
		},
	}
}

var tuning = session.TuningProfile{TickRate: 60, SendRate: 60, HistoryDepth: 4, SnapshotEvery: 0}

func pair(t *testing.T, p session.TuningProfile) (*statesync.Sender[world, delta], *statesync.Receiver[world, delta]) {
	t.Helper()
	snd, err := statesync.NewSender(codec(), p)
	if err != nil {
		t.Fatal(err)
	}
	rcv, err := statesync.NewReceiver(codec(), p)
	if err != nil {
		t.Fatal(err)
	}
	return snd, rcv
}

// The delta chain reconstructs exactly the states the sender committed.
func TestDeltaChainReconstructs(t *testing.T) {
	snd, rcv := pair(t, tuning)
	w := world{}
	for tick := session.Tick(1); tick <= 20; tick++ {
		w.A = int64(tick) * 3
		if tick%4 == 0 {
			w.B++
		}
		pkt := snd.Send(tick, &w)
		if tick == 1 && pkt.Kind != statesync.KindSnapshot {
			t.Fatalf("first send must be a snapshot, got %d", pkt.Kind)
		}
		if tick > 1 && pkt.Kind != statesync.KindDelta {
			t.Fatalf("tick %d: expected delta, got %d", tick, pkt.Kind)
		}
		if err := rcv.Apply(pkt); err != nil {
			t.Fatal(err)
		}
		got, gotTick, ok := rcv.State()
		if !ok || gotTick != tick || *got != w {
			t.Fatalf("tick %d: receiver has %+v at %d, want %+v", tick, got, gotTick, w)
		}
	}
}

// A lost packet breaks the speculative chain: the receiver rejects the
// next delta, the sender is told, and a snapshot resynchronizes
// (rule:delta-baseline-must-be-retained, concept:delta-baseline-policy
// fallback).
func TestLossForcesResync(t *testing.T) {
	snd, rcv := pair(t, tuning)
	w := world{A: 1}
	if err := rcv.Apply(snd.Send(1, &w)); err != nil {
		t.Fatal(err)
	}
	w.A = 2
	_ = snd.Send(2, &w) // lost in transit
	w.A = 3
	pkt := snd.Send(3, &w)
	err := rcv.Apply(pkt)
	if !errors.Is(err, statesync.ErrResyncNeeded) {
		t.Fatalf("expected ErrResyncNeeded, got %v", err)
	}
	// Receiver state unchanged by the rejected delta.
	got, gotTick, _ := rcv.State()
	if got.A != 1 || gotTick != 1 {
		t.Fatalf("rejected delta mutated receiver: %+v at %d", got, gotTick)
	}
	snd.ResyncRequested()
	w.A = 4
	pkt = snd.Send(4, &w)
	if pkt.Kind != statesync.KindSnapshot {
		t.Fatalf("post-resync send must be a snapshot, got %d", pkt.Kind)
	}
	if err := rcv.Apply(pkt); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := rcv.State(); got.A != 4 {
		t.Fatalf("resync failed: %+v", got)
	}
}

// The snapshot cadence inserts full states among the deltas.
func TestSnapshotCadence(t *testing.T) {
	p := tuning
	p.SnapshotEvery = 3
	snd, rcv := pair(t, p)
	w := world{}
	kinds := []statesync.Kind{}
	for tick := session.Tick(1); tick <= 8; tick++ {
		w.A = int64(tick)
		pkt := snd.Send(tick, &w)
		kinds = append(kinds, pkt.Kind)
		if err := rcv.Apply(pkt); err != nil {
			t.Fatal(err)
		}
	}
	// snapshot, then 3 deltas, snapshot, 3 deltas...
	want := []statesync.Kind{1, 2, 2, 2, 1, 2, 2, 2}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
}
