package runconf

import (
	"strings"
	"testing"
)

// valid is the shape Load produces from an empty file: every default in
// force. Each case below breaks exactly one thing.
func valid() Run {
	return Run{
		Topology:          "standalone",
		Sync:              Sync{Mode: "server_authoritative", Baseline: "speculative", Ack: "piggyback_only"},
		Time:              Time{Mode: "realtime", ScalePermille: 1000},
		Episode:           Episode{Mode: "analysis_sampled", SamplePercent: 100},
		Debug:             Debug{Listen: "127.0.0.1:8932"},
		EvaluationVersion: 1,
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
		{"unlisted sync mode", func(r *Run) { r.Sync.Mode = "lockstep" }, "run.sync.mode"},
		{"unlisted ack mode", func(r *Run) { r.Sync.Ack = "always" }, "run.sync.ack"},
		{"listen topology without an address", func(r *Run) { r.Topology = "listen" }, "run.listen"},
		{"standalone with an address", func(r *Run) { r.Listen = "0.0.0.0:1" }, "run.listen"},
		{"bounded speculation without a depth", func(r *Run) {
			r.Sync.Baseline = "bounded_speculation"
		}, "speculation_depth"},
		{"speculation depth on another baseline", func(r *Run) {
			r.Sync.SpeculationDepth = 3
		}, "speculation_depth"},
		{"scaled clock without a scale", func(r *Run) {
			r.Time.Mode = "scaled"
			r.Time.ScalePermille = 0
		}, "scale_permille"},
		{"unknown controller kind", func(r *Run) {
			r.Slot = []Slot{{Index: 0, Kind: "oracle"}}
		}, "kind"},
		{"controller without its source", func(r *Run) {
			r.Slot = []Slot{{Index: 0, Kind: "replay"}}
		}, "needs a source"},
		{"two controllers on one slot", func(r *Run) {
			r.Slot = []Slot{{Index: 1, Kind: "human"}, {Index: 1, Kind: "remote"}}
		}, "repeats index"},
		{"sampling out of range", func(r *Run) {
			r.Episode.Dir = "corpus"
			r.Episode.SamplePercent = 0
		}, "sample_percent"},
		{"sampled replay corpus", func(r *Run) {
			r.Episode.Dir = "corpus"
			r.Episode.Mode = "replay_complete"
			r.Episode.SamplePercent = 10
		}, "replay_complete"},
		{"evaluation version zero", func(r *Run) { r.EvaluationVersion = 0 }, "evaluation_version"},
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

// Episode settings are inert while nothing is recorded, so an unset block
// must not fail a run that records nothing.
func TestEpisodeBlockIgnoredWhenRecordingIsOff(t *testing.T) {
	r := valid()
	r.Episode = Episode{Mode: "", SamplePercent: 0}
	if err := r.Validate(); err != nil {
		t.Fatalf("recording off should not validate the block: %v", err)
	}
}

// A player can hold a session of any size: four rows of
// concept:deployment-combination carry a playing host at "2 or many"
// seats, and a browser hosting a party over WebRTC with no backend is
// concept:static-host-mode. Nothing here may tie the host to the seat
// count.
func TestAnyTopologyRunsAtAnySeatCount(t *testing.T) {
	for _, topology := range topologies {
		r := valid()
		r.Topology = topology
		if topology != "standalone" {
			r.Listen = "0.0.0.0:4433"
		}
		r.Slot = []Slot{
			{Index: 0, Kind: "human"},
			{Index: 1, Kind: "remote"},
			{Index: 2, Kind: "remote"},
			{Index: 3, Kind: "remote"},
			{Index: 4, Kind: "remote"},
			{Index: 5, Kind: "remote"},
		}
		if err := r.Validate(); err != nil {
			t.Errorf("topology %q with six seats: %v", topology, err)
		}
	}
}
