package game

import (
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/msg"
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

// Binding hands the rules to the wrapper, with the seat left unfilled.
//
// Step 3 named the hand-written bot here. Step 4 cannot, and the reason
// is a layering fact rather than an inconvenience: the distilled agent
// calls predicates that judge a Sight, so the generated package depends
// on this one. A rules package that reached back for it would be
// depending on something distilled from its own recordings, and the
// compiler says so — the import cycle is the design being enforced.
//
// So the choice moves to the entry point, beside the transport, for the
// same reason the transport is there. Which opponent this build seats is
// a property of what was generated for it, not of the rules.
func Binding() run.Binding[msg.TTTWorld, msg.Move, Sight] {
	return run.Binding[msg.TTTWorld, msg.Move, Sight]{
		Slots:             Slots(),
		Config:            Config,
		ProtocolVersion:   Protocol,
		EvaluationVersion: Evaluation,
	}
}

// HandWritten is the bot the corpus was recorded from, offered as a
// factory so an entry point or a test can seat it the same way it seats
// the generated one.
func HandWritten(session.SlotID) (string, session.Agent[Sight, msg.Move]) {
	return "tactic", &Bot{}
}
