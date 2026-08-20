package rtslite_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/samples/rtslite/msg"
	"github.com/shibukawa/ebigentserver/samples/rtslite/rtslite"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
)

// Phase 6 acceptance: on a world this large, the modes of
// concept:delta-baseline-policy diverge measurably. The same battle's
// committed states stream through one fog-projected sender per mode,
// with the receiver's acks arriving several sends late (a realistic
// RTT); the measurement is total bytes on the wire.
//
// What the numbers show: speculative diffs against the last send
// (smallest deltas, but a single loss breaks the chain until resync);
// confirmed_only diffs against the ack-lagged baseline (every packet
// decodable on arrival, paid for with deltas that grow with the lag);
// bounded speculation sits between, by declaration.
func TestBaselineModesDivergeMeasurably(t *testing.T) {
	// One deterministic 4-player battle, states collected at the send
	// cadence.
	g := rtslite.Game{Players: 4, TickLimit: 600}
	s := g.Start(9)
	// Everyone charges the center: maximal churn in every fog view.
	for slot := session.SlotID(1); slot <= 4; slot++ {
		for _, u := range s.Units {
			if msg.OwnerOf(u.ID) == uint16(slot) {
				g.Apply(&s, slot, rtslite.Input{Unit: u.ID, TargetX: msg.MapW/2 + uint8(slot), TargetY: msg.MapH / 2})
			}
		}
	}
	codec := rtslite.ViewCodec()
	var states []rtslite.State
	const sendEvery = 2
	for i := 0; i < 300 && !s.Over; i++ {
		g.Advance(&s)
		if i%sendEvery == 0 {
			states = append(states, codecCloneState(&s))
		}
	}
	if len(states) < 50 {
		t.Fatalf("battle too short: %d send states", len(states))
	}

	const ackLagSends = 6 // ~RTT of 6 send intervals
	type result struct {
		name          string
		mode          session.BaselineMode
		bytes         int
		deltas, snaps int
		maxDelta      int
	}
	// The ack pattern: a steady lag of ackLagSends, plus an RTT spike
	// (no acks at all for a stretch) mid-battle. During the spike,
	// bounded speculation falls back to the confirmed baseline while
	// speculative sails on; confirmed_only pays the lag the whole time.
	stallFrom, stallTo := 60, 90
	results := []result{
		{name: "speculative", mode: session.BaselineSpeculative},
		{name: "bounded(16)", mode: session.BaselineBounded},
		{name: "confirmed_only", mode: session.BaselineConfirmedOnly},
	}
	for i := range results {
		r := &results[i]
		tuning := session.TuningProfile{
			TickRate: 30, SendRate: 15, HistoryDepth: 64, SnapshotEvery: 0,
			BaselineMode: r.mode, SpeculationDepth: 16,
		}
		if r.mode != session.BaselineBounded {
			tuning.SpeculationDepth = 0
		}
		if err := tuning.Validate(); err != nil {
			t.Fatal(err)
		}
		snd, err := statesync.NewProjectedSender(codec, tuning, func(st *rtslite.State) msg.PlayerView {
			return rtslite.ProjectPlayer(st, 1)
		})
		if err != nil {
			t.Fatal(err)
		}
		var inflight []session.Tick
		for si, st := range states {
			tick := session.Tick(st.Tick)
			stalled := si >= stallFrom && si < stallTo
			for !stalled && len(inflight) >= ackLagSends {
				snd.Confirm(inflight[0])
				inflight = inflight[1:]
			}
			pkt := snd.Send(tick, &st)
			n := len(pkt.Payload) + 17
			r.bytes += n
			if pkt.Kind == statesync.KindDelta {
				r.deltas++
				if n > r.maxDelta {
					r.maxDelta = n
				}
			} else {
				r.snaps++
			}
			inflight = append(inflight, tick)
		}
	}

	for _, r := range results {
		t.Logf("%-14s total %6dB  deltas %3d (max %4dB)  snapshots %d",
			r.name, r.bytes, r.deltas, r.maxDelta, r.snaps)
	}
	spec, bounded, conf := results[0], results[1], results[2]
	if !(spec.bytes < bounded.bytes) {
		t.Errorf("speculative (%dB) not cheaper than bounded (%dB)", spec.bytes, bounded.bytes)
	}
	if !(spec.bytes < conf.bytes) {
		t.Errorf("speculative (%dB) not cheaper than confirmed_only (%dB)", spec.bytes, conf.bytes)
	}
	if !(bounded.bytes < conf.bytes) {
		t.Errorf("bounded (%dB) not cheaper than confirmed_only (%dB)", bounded.bytes, conf.bytes)
	}
	if conf.bytes < spec.bytes*11/10 {
		t.Errorf("modes did not diverge measurably: confirmed %dB vs speculative %dB", conf.bytes, spec.bytes)
	}
}

func codecCloneState(s *rtslite.State) rtslite.State {
	c := *s
	c.Units = append([]msg.Unit(nil), s.Units...)
	return c
}
