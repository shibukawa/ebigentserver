// Package runconf declares data:run-config: the run sections of
// ebigent.toml that a built artifact reads at startup
// (decision:one-config-file-many-sections).
//
// Everything here is chosen per process launch, and nothing else is. What
// the game settles about itself — how many play, how they connect, which
// concept:synchronization-mode the rules assume — is the protocol level
// of concept:configuration-scope, emitted as constants by ebigent
// generate and never read from a file (rule:config-tier-placement).
//
// Both the ebigent tool and the built artifact bind these sections; each
// ignores the prefixes it does not own, which is what lets one file serve
// both readers. A deployment may also point an artifact at a file holding
// nothing but these sections, since those are the only ones it binds.
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
	// Listen is the address a host binds. Standalone opens no listener,
	// and a p2p peer dials rather than binds.
	Listen string `default:"" dependon:".topology=listen,dedicated" help:"realtime listen address as host:port"`
	// Server is where a process that does not hold the session goes to
	// find one. It is the other half of Listen: one binds, this dials,
	// and a deployment that moves its server changes this and nothing
	// else.
	Server string `default:"" help:"address of the host to join, as host:port or a url; empty discovers or hosts"`
	// Transport is what this process may open.
	Transport Transport `dependon:".topology!=standalone" help:"Transport is what this process may open"`
	// Time is concept:game-time-control.
	Time Time `help:"Time is concept:game-time-control"`
	// Tuning is data:session-tuning-profile, the timing and bandwidth
	// values of one stage.
	Tuning Tuning `help:"Tuning is data:session-tuning-profile, the timing and bandwidth values of one stage"`
	// Debug is api:dev-debug-endpoint, honored only by a development
	// build.
	Debug Debug `help:"Debug is api:dev-debug-endpoint, honored only by a development build"`
	// Episode is what this process records and how much of it it plays.
	Episode Episode `help:"Episode is what this process records and how much of it it plays"`
}

// Episode is the data:episode-log this process writes: where it goes,
// how much of it is kept, and how many matches are played before the
// process exits.
//
// It is a run setting rather than a build one because two launches of
// one artifact legitimately differ on all of it: the same headless
// binary is a training run of four hundred matches on a workstation and
// a dedicated server that records nothing.
//
// This is also the channel `ebigent simulate` uses. The tool holds the
// project's corpus root and the match count a person asked for, and it
// reaches the child through the environment layer of this same binding
// rather than through a convention invented for the occasion.
type Episode struct {
	// Root is the corpus directory. Empty records nothing at zero cost,
	// which is what a server wants.
	Root string `default:"" help:"corpus root; empty records nothing"`
	// Mode is concept:episode-recording-mode. A corpus is the common
	// case; a bit-exact replay log is the deliberate one.
	Mode string `default:"analysis_sampled" enum:"analysis_sampled,replay_complete" help:"how much of each episode is kept"`
	// Matches is how many to play before exiting. 0 plays until the
	// process is interrupted, which is what a server does.
	Matches int `default:"1" help:"matches to play before exiting; 0 plays until interrupted"`
	// Seed is the first match's seed; each later match adds its index,
	// so a whole run reproduces from this one number
	// (rule:shared-rng-seed).
	Seed int `default:"1" help:"seed of the first match; each later match adds its index"`
}

// Transport declares the listeners this deployment may open. Selection
// among them is by concept:transport-capability and never by protocol
// name (rule:transport-selected-by-capability); naming them here is a
// deployment fact, not selection logic.
//
// What the game *requires* of a transport is not here: it follows from
// the realtime intensity of the protocol level and is emitted with it.
type Transport struct {
	// Enable lists the listeners this process may open. Empty leaves the
	// choice to whatever the build linked.
	Enable []string `help:"listeners this process may open: websocket, webtransport, webrtc"`
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

// Tuning is data:session-tuning-profile bound per run.
//
// Every peer of one match must bind the same values. netplay's client
// declares the profile the server runs and reconstructs state against it,
// so this is a deployment-wide setting rather than a per-machine one —
// two peers disagreeing here do not negotiate, they desynchronize.
type Tuning struct {
	// TickRate is simulation steps per second.
	TickRate int `default:"60" help:"simulation steps per second"`
	// SendRate is downstream updates per second, at most TickRate, and
	// TickRate must be a whole multiple of it.
	SendRate int `default:"30" help:"state updates per second; tick_rate must be a whole multiple"`
	// SnapshotEvery sends a full data:snapshot every N-th update; 0
	// sends one only on join and resync.
	SnapshotEvery int `default:"120" help:"full snapshot every N updates; 0 sends one only on join"`
	// HistoryDepth is how many committed world versions the sender
	// retains per receiver, bounding every delta baseline
	// (rule:delta-baseline-must-be-retained).
	HistoryDepth int `default:"12" help:"retained world versions per receiver"`
	// Baseline is concept:delta-baseline-policy.
	Baseline string `default:"speculative" enum:"speculative,confirmed_only,bounded_speculation" help:"which retained version deltas are computed against"`
	// SpeculationDepth bounds bounded_speculation; it is required for
	// that baseline and refused by the others.
	SpeculationDepth int `default:"0" dependon:".baseline=bounded_speculation" help:"unconfirmed versions bounded_speculation may speculate past the confirmed baseline"`
	// Ack is concept:ack-transmission-policy.
	Ack string `default:"piggyback_only" enum:"piggyback_only,dedicated,delayed_piggyback" help:"how ack records reach the peer"`
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
