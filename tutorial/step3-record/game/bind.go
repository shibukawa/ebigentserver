package game

import (
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/tutorial/step3-record/msg"
)

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
// NewAgent is what step 2 left blank, and filling it in is the whole of
// what it takes to have an opponent. Nothing else moved: no rule learned
// what a bot is, no branch anywhere asks who occupies a seat, and the
// window build is not aware that this line exists.
//
// The same factory serves two callers who could not look less alike. A
// press in the lobby seats a stand-in for the person who never arrived;
// a headless run fills every seat from here and plays with nobody
// watching. Both are the same act — put an agent in a slot — which is
// why one function answers both.
func Binding() run.Binding[msg.TTTWorld, msg.Move, Sight] {
	return run.Binding[msg.TTTWorld, msg.Move, Sight]{
		Slots:             Slots(),
		Config:            Config,
		NewAgent:          NewBot,
		ProtocolVersion:   Protocol,
		EvaluationVersion: Evaluation,
	}
}
