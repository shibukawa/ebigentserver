package runconf

import (
	"strings"
	"testing"
)

// valid is the shape Load produces from an empty file: every default in
// force. Each case below breaks exactly one thing.
func valid() Run {
	return Run{
		Topology: "standalone",
		Time:     Time{Mode: "realtime", ScalePermille: 1000},
		Tuning: Tuning{
			TickRate: 60, SendRate: 30, SnapshotEvery: 120, HistoryDepth: 12,
			Baseline: "speculative", Ack: "piggyback_only",
		},
		Debug: Debug{Listen: "127.0.0.1:8932"},
	}
}

func TestDefaultsValidate(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("the default run should be valid: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Run)
		want string
	}{
		{"unlisted topology", func(r *Run) { r.Topology = "peer2peer" }, "run.topology"},
		{"unlisted ack mode", func(r *Run) { r.Tuning.Ack = "always" }, "run.tuning.ack"},
		{"unlisted baseline", func(r *Run) { r.Tuning.Baseline = "hopeful" }, "run.tuning.baseline"},
		{"listen topology without an address", func(r *Run) { r.Topology = "listen" }, "run.listen"},
		{"standalone with an address", func(r *Run) { r.Listen = "0.0.0.0:1" }, "run.listen"},
		{"standalone dialing somebody", func(r *Run) { r.Server = "host:4433" }, "run.server"},
		{"binding and dialing at once", func(r *Run) {
			r.Topology = "listen"
			r.Listen, r.Server = "0.0.0.0:1", "host:4433"
		}, "two sides of one link"},
		{"bounded speculation without a depth", func(r *Run) {
			r.Tuning.Baseline = "bounded_speculation"
		}, "speculation_depth"},
		{"speculation depth on another baseline", func(r *Run) {
			r.Tuning.SpeculationDepth = 3
		}, "speculation_depth"},
		{"speculating past what is retained", func(r *Run) {
			r.Tuning.Baseline = "bounded_speculation"
			r.Tuning.SpeculationDepth, r.Tuning.HistoryDepth = 12, 12
		}, "below history_depth"},
		{"scaled clock without a scale", func(r *Run) {
			r.Time.Mode = "scaled"
			r.Time.ScalePermille = 0
		}, "scale_permille"},
		{"a session that never steps", func(r *Run) { r.Tuning.TickRate = 0 }, "tick_rate"},
		{"a session nobody hears", func(r *Run) { r.Tuning.SendRate = 0 }, "send_rate"},
		{"sending faster than it steps", func(r *Run) {
			r.Tuning.TickRate, r.Tuning.SendRate = 30, 60
		}, "above tick_rate"},
		{"a cadence that lands between ticks", func(r *Run) {
			r.Tuning.TickRate, r.Tuning.SendRate = 60, 45
		}, "whole multiple"},
		{"no retained baseline", func(r *Run) { r.Tuning.HistoryDepth = 0 }, "history_depth"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := valid()
			tc.mut(&r)
			err := r.Validate()
			if err == nil {
				t.Fatalf("want an error naming %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to name %q", err, tc.want)
			}
		})
	}
}

// A player can hold a session of any size: four rows of
// concept:deployment-combination carry a playing host at "2 or many"
// seats, and a browser hosting a party over WebRTC with no backend is
// concept:static-host-mode. Nothing here may tie the host to the seat
// count — which is now structural, since the seat composition is the
// protocol level of concept:configuration-scope and this table cannot
// see it at all.
func TestEveryTopologyValidates(t *testing.T) {
	for _, topology := range topologies {
		r := valid()
		r.Topology = topology
		if topology == "listen" || topology == "dedicated" {
			r.Listen = "0.0.0.0:4433"
		}
		if err := r.Validate(); err != nil {
			t.Errorf("topology %q: %v", topology, err)
		}
	}
}

// A client reaching a dedicated server is the case the table could not
// express before: Listen binds and Server dials, and a deployment that
// moves its server changes the one key.
func TestAClientDialsWithoutBinding(t *testing.T) {
	r := valid()
	r.Topology = "p2p"
	r.Server = "game.example.com:4433"
	if err := r.Validate(); err != nil {
		t.Fatalf("dialing a host: %v", err)
	}
}
