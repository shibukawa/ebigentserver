package game

import (
	"time"

	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/run/lan"
	"github.com/shibukawa/ebigentserver/tutorial/step2-lobby/msg"
)

// This file is everything the wrapper needs to know about these rules,
// and it lives beside them so the window build and any later headless
// build start from the same declaration and cannot drift.
//
// Importing run and run/lan here is not a boundary violation: neither
// links an engine. The engine half is run/eb, and only main.go imports
// that (rule:engine-import-confined-to-client-entry).

// Options is what this game declares to api:run-wrapper. Two seats, one
// of them at this keyboard, and a mouse to place with.
func Options() run.Options {
	return run.Options{
		Name:          "tictactoe",
		Devices:       run.Mouse,
		MaxLocalSeats: 1,
	}
}

// Binding hands the rules to the wrapper.
//
// NewAgent is absent on purpose. The empty seat is not a seat for a bot:
// it belongs to the other person, who cannot exist until this game is
// already running. Step 3 is where a bot gets a name.
func Binding() run.Binding[State, Action, Observation] {
	return run.Binding[State, Action, Observation]{
		Slots:             Slots(),
		Config:            Config,
		ProtocolVersion:   Protocol,
		EvaluationVersion: Evaluation,
	}
}

// Network is the LAN preset: look for a game on this network for a
// moment, join it if one answers, and offer one if none does.
//
// That is the whole configuration. Two people launch the same binary and
// the first to start hosts, because nobody answered it.
func Network() run.Networking[State, Action, Observation] {
	return lan.Auto(LANOptions(), 1500*time.Millisecond)
}

// LANOptions declares what goes on the wire. The preset encodes nothing
// itself; every byte comes from the generated codec above.
func LANOptions() lan.Options[State, Action, msg.TTTStateDelta, Observation] {
	return lan.Options[State, Action, msg.TTTStateDelta, Observation]{
		Name:        "tictactoe",
		Protocol:    Protocol,
		Codec:       Codec(),
		Tuning:      Tuning(),
		EncodeInput: EncodeAction,
		DecodeInput: DecodeAction,
		Project:     Simulation{}.Project,
	}
}
