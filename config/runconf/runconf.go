// Package runconf declares data:run-config: the run sections of
// ebigent.toml that a built artifact reads at startup
// (decision:one-config-file-many-sections).
//
// Everything here is chosen per process launch. What is fixed at link
// time belongs to concept:build-target instead, and the timing and
// bandwidth constants of one game belong to session.TuningProfile, which
// a run never overrides (data:run-config: "the game profile is never
// overridden by a run").
//
// Both the ebigent tool and the built artifact bind these sections; each
// ignores the prefixes it does not own, which is what lets one file
// serve both readers.
package runconf

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

import "github.com/shibukawa/tinybind-go/configbind"

// Prefix is the configuration prefix these sections live under.
const Prefix = "run"

// Run is the [run] table. Field order is the order provenance reports.
type Run struct {
	// Topology is concept:execution-topology: where this process places
	// the session and its agents.
	Topology string `default:"standalone" enum:"standalone,listen,dedicated,p2p" help:"execution topology of this process"`
	// Listen is the realtime endpoint address. Standalone has no
	// listener, and a p2p peer dials rather than binds.
	Listen string `default:"" dependon:".topology=listen,dedicated" help:"realtime listen address as host:port"`
	// Transport is what this process must be able to carry.
	Transport Transport `dependon:".topology!=standalone" help:"Transport is what this process must be able to carry"`
	// Sync selects the synchronization behavior of the session.
	Sync Sync `help:"Sync selects the synchronization behavior of the session"`
	// Time is concept:game-time-control.
	Time Time `help:"Time is concept:game-time-control"`
	// Slot is the agent roster: which controller fills each
	// concept:player-slot. An array of tables, so the file is its only
	// source — element fields have no CLI option and no env var.
	Slot []Slot `help:"Slot is the agent roster: which controller fills each concept:player-slot. An array of tables, so the file is its only source — element fields have no CLI option and no env var"`
	// Episode is data:episode-log recording.
	Episode Episode `help:"Episode is data:episode-log recording"`
	// Debug is api:dev-debug-endpoint, honored only by a development
	// build.
	Debug Debug `help:"Debug is api:dev-debug-endpoint, honored only by a development build"`
	// EvaluationVersion is the data:evaluation-signal version this run
	// reports, so a corpus can be filtered by the scoring it was
	// produced under.
	EvaluationVersion int `default:"1" summary:"omit" help:"evaluation signal version recorded with every episode"`
}

// Transport declares the delivery classes this deployment needs and the
// listeners it may open. Selection among the enabled listeners is by
// concept:transport-capability and never by protocol name
// (rule:transport-selected-by-capability); naming them here is a
// deployment fact, not selection logic.
type Transport struct {
	// Enable lists the listeners this process may open. Empty leaves
	// the choice to whatever the build linked.
	Enable []string `help:"listeners this process may open: websocket, webtransport, webrtc"`
	// RequireUnreliable rejects a deployment whose enabled listeners
	// offer no unreliable datagram channel.
	RequireUnreliable bool `default:"false" help:"require an unreliable datagram channel"`
	// RequirePeerToPeer rejects a deployment that cannot connect
	// without a server in the middle.
	RequirePeerToPeer bool `default:"false" help:"require a peer to peer capable transport"`
	// RequireBrowser rejects a deployment unreachable from a browser
	// build.
	RequireBrowser bool `default:"false" help:"require a browser reachable transport"`
}

// Sync is the per-run half of concept:synchronization-mode and the two
// policies that ride with it. The values a game cannot change at runtime
// stay in session.TuningProfile.
type Sync struct {
	// Mode is concept:synchronization-mode.
	Mode string `default:"server_authoritative" enum:"delay,rollback,server_authoritative,hybrid" help:"synchronization mode"`
	// Baseline is concept:delta-baseline-policy. The adaptive mode of
	// the concept has no implementation yet and is not offered here.
	Baseline string `default:"speculative" enum:"speculative,confirmed_only,bounded_speculation" help:"which retained version deltas are computed against"`
	// SpeculationDepth bounds bounded_speculation; it is required for
	// that baseline and ignored by the others.
	SpeculationDepth int `default:"0" dependon:".baseline=bounded_speculation" help:"unconfirmed versions bounded_speculation may speculate past the confirmed baseline"`
	// Ack is concept:ack-transmission-policy.
	Ack string `default:"piggyback_only" enum:"piggyback_only,dedicated,delayed_piggyback" help:"how ack records reach the peer"`
}

// Time is concept:game-time-control: the session clock decoupled from the
// wall clock so a slow agent can still participate.
type Time struct {
	// Mode selects the clock policy. Step advances only when the agent
	// signals completion (decision:dual-mode-agent-pacing).
	Mode string `default:"realtime" enum:"realtime,scaled,step,unlimited" help:"session clock policy"`
	// ScalePermille is the clock rate in thousandths, so 1000 is
	// realtime and 100 is 0.1x. Thousandths rather than a float because
	// configbind binds no float type and rule:no-float-in-simulation
	// keeps reals out of the simulation path regardless.
	ScalePermille int `default:"1000" dependon:".mode=scaled" help:"clock rate in thousandths, 1000 being realtime"`
}

// Slot is one entry of the agent roster: which controller fills one
// concept:player-slot. Element of an array of tables, so it carries no
// enum, falsy, or dependon tag — those need a stable config key, and an
// element key belongs to one element rather than to the configuration.
type Slot struct {
	// Index is the concept:player-slot this entry fills.
	Index int `default:"0" help:"player slot index"`
	// Kind selects the actor: human, script, behavior_tree, llm,
	// replay, or remote. Validated by Validate rather than by an enum
	// tag.
	Kind string `default:"remote" help:"controller kind: human, script, behavior_tree, llm, replay, remote"`
	// Source is what the controller reads: a chip library for
	// behavior_tree, an episode for replay, a script name for script.
	// Unused by human and remote.
	Source string `default:"" help:"controller input: chip library, episode path, or script name"`
}

// Episode is data:episode-log recording for this run.
type Episode struct {
	// Dir is the recording destination. Empty records nothing, which
	// is what hides the rest of this block from provenance.
	Dir string `default:"" help:"episode recording directory; empty disables recording"`
	// Mode is concept:episode-recording-mode.
	Mode string `default:"analysis_sampled" enum:"replay_complete,analysis_sampled" dependon:".dir" help:"recording contract"`
	// SamplePercent is how many sessions are recorded, out of a
	// hundred. replay_complete still records every tick of the sessions
	// it selects; sampling chooses sessions, never ticks.
	SamplePercent int `default:"100" dependon:".dir" help:"percent of sessions recorded"`
}

// Debug is api:dev-debug-endpoint. The address is a run value, but the
// endpoint itself is a linkage decision: only a development entry point
// imports it (rule:debug-endpoint-excluded-from-release), so setting this
// in a release artifact does nothing rather than opening a port.
type Debug struct {
	// Listen is the endpoint address. Loopback by default so an
	// inspection channel exposing full concept:world-state is never
	// bound to a routable interface by accident. Empty disables it.
	Listen string `default:"127.0.0.1:8932" help:"dev debug endpoint address; empty disables it"`
}

// Bind registers the run sections and returns the destination the next
// configbind.Load fills. Call it before Load, never after.
func Bind() *Run { return configbind.Bind[Run](Prefix) }

// Prefixes reports the configuration prefixes Bind claims, for the
// prefix-scoped stray key check of decision:one-config-file-many-sections.
func Prefixes() []string { return []string{Prefix} }
