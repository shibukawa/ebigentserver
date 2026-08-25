package game

import (
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// This file is everything the wrapper needs to know about these rules. It
// lives with the rules rather than in an entry point so both builds —
// the one with a window and the one without — start from the same
// declaration and cannot drift.
//
// Importing run from here is not a boundary violation: run links no
// engine. The engine half is run/eb, and only the client entry imports
// that (rule:engine-import-confined-to-client-entry).

// Protocol identifies this game's message schema in every episode header,
// so a corpus cannot silently mix runs of incompatible rules.
const Protocol = "solo-1"

// Evaluation versions the scoring in Evaluate. Changing how a slot is
// scored invalidates comparisons across a corpus, so the number moves
// with it.
const Evaluation = 1

// Options is what this game declares to api:run-wrapper.
//
// One person plays, so one local seat, and the enemy seats are agents in
// the same process rather than people at the same screen — which is why
// SharedScreen is false even though four seats run here.
func Options() run.Options {
	return run.Options{
		Name:          "solo",
		Devices:       run.Keyboard | run.Gamepad,
		MaxLocalSeats: 1,
	}
}

// Binding hands the rules to the wrapper.
func Binding() run.Binding[State, Action, Sight] {
	return run.Binding[State, Action, Sight]{
		Slots:    Slots(),
		Config:   Config,
		NewAgent: NewAgent,
		// The same three kinds NewAgent chooses between, named so a run
		// can ask for one. Recording them separately is the point: a
		// corpus mixing three pursuit styles distills into a policy
		// none of them had.
		Agents: map[string]func() session.Agent[Sight, Action]{
			KindRunner:  func() session.Agent[Sight, Action] { return &Runner{} },
			KindChaser:  func() session.Agent[Sight, Action] { return &Chaser{} },
			KindFlanker: func() session.Agent[Sight, Action] { return &Flanker{} },
		},
		ProtocolVersion:   Protocol,
		EvaluationVersion: Evaluation,
	}
}
