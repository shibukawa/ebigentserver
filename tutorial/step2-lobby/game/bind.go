package game

import "github.com/shibukawa/ebigentserver/run"

// This file is everything the wrapper needs to know about these rules,
// and it lives beside them so the window build and any later headless
// build start from the same declaration and cannot drift.
//
// Importing run here is not a boundary violation: it links no engine.
// The engine half is run/eb, and only main.go imports that
// (rule:engine-import-confined-to-client-entry).
//
// What is deliberately absent is the network. Which transport reaches
// the other player is a property of where this build runs, not of the
// rules — so it is declared at the entry point, beside the window, and
// these rules stay buildable for every target.

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
func Binding() run.Binding[World, Action, Sight] {
	return run.Binding[World, Action, Sight]{
		Slots:             Slots(),
		Config:            Config,
		ProtocolVersion:   Protocol,
		EvaluationVersion: Evaluation,
	}
}
