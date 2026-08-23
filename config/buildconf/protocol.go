package buildconf

// Protocol is the [protocol] table: the game's own contract, and the
// protocol level of concept:configuration-scope.
//
// How many play, how they connect, and how tightly the loop has to close
// are the terms every participant agrees to before anything runs. None of
// it differs between a development machine and production, so none of it
// is read at startup: ebigent generate turns this table into Go
// constants and the artifact carries them (rule:config-tier-placement).
//
// Every field here is either asked by flow:project-init or settled by the
// answers it already has.
type Protocol struct {
	// Package is the import path of the game, subpath included, and half
	// of the identity pair of decision:module-path-is-game-identity.
	// Empty takes project.module, which is the whole path when one
	// module holds one game.
	Package string `default:"" help:"import path of the game, subpath included; empty takes project.module"`
	// Title is what a browse list and a window caption show. It
	// identifies nothing, so it may be reworded or localized freely.
	Title string `default:"" help:"display name; empty takes the last element of the package path"`
	// Shape is concept:participant-shape.
	Shape string `default:"solo" enum:"solo,duo,multi" help:"solo, duo, or multi"`
	// Realtime is concept:realtime-intensity, which decides the
	// concept:transport-capability this game needs.
	Realtime string `default:"paced" enum:"turn_based,paced,twitch" help:"turn_based, paced, or twitch"`
	// View is concept:view-arrangement: whether every seat reads the
	// same screen content or holds a view of its own.
	View string `default:"shared" enum:"shared,per_agent" help:"shared or per_agent"`
	// Devices are the input devices this build accepts. A game cannot
	// accept one it never wrote an api:input-adapter for.
	Devices []string `help:"accepted input devices: keyboard, mouse, gamepad"`
	// Sync is concept:synchronization-mode. It moved here from the run
	// sections because a mode is settled at build, and offering it per
	// launch is the mistake rule:build-tag-only-for-linkage names in its
	// own domain.
	Sync string `default:"server_authoritative" enum:"server_authoritative,delay,rollback,hybrid" help:"synchronization mode"`
	// Seats is the seat composition api:roster fills.
	Seats Seats `help:"Seats is the seat composition api:roster fills"`
	// Team is the division of the seats. No entry means no teams; the
	// seat counts must add up to Seats.Count when there are any.
	Team []Team `help:"Team is the division of the seats. No entry means no teams; the seat counts must add up to Seats.Count when there are any"`
	// Condition is the axis set matchmaking may filter on. No entry
	// means a room states nothing beyond its identity and version.
	Condition []Condition `help:"Condition is the axis set matchmaking may filter on. No entry means a room states nothing beyond its identity and version"`
}

// Seats is the [protocol.seats] table: how many seats there are and what
// may fill them.
type Seats struct {
	// Count is how many concept:player-slot entries the rules declare.
	Count int `default:"1" help:"number of declared seats"`
	// Fill decides what a match does with a seat nobody took. Bots
	// completes the roster so play can start; empty starts short.
	Fill string `default:"bots" enum:"bots,empty" help:"fill an unclaimed seat with a bot, or start with it empty"`
	// Occupant bounds who may take a seat, unless a team narrows it
	// further.
	Occupant string `default:"any" enum:"any,human,bot" help:"who may take a seat: any, human, or bot"`
}

// Team is one [[protocol.team]] block. Element of an array of tables, so
// it carries no enum tag and Validate checks Occupant instead.
type Team struct {
	// Name labels the team in the lobby and in data:episode-log.
	Name string `default:"" help:"team name"`
	// Seats is how many of the declared seats belong to it.
	Seats int `default:"0" help:"seats on this team"`
	// Occupant narrows Seats.Occupant for this team alone. Empty takes
	// the table's value.
	Occupant string `default:"" help:"who may take a seat on this team: any, human, or bot; empty takes protocol.seats.occupant"`
}

// Condition is one [[protocol.condition]] block: an axis matchmaking may
// filter on.
//
// The axes are declared at build and the values are chosen per room
// (requirement:conditional-matchmaking). Two ends that disagree about
// which axes exist cannot explain a refusal to each other, which is the
// same reason a schema is settled at build rather than per launch.
//
// Element of an array of tables, so it carries no enum tag and Validate
// checks Match instead.
type Condition struct {
	// Name is the axis, and the key a room states a value under.
	Name string `default:"" help:"axis name, such as mode or rank"`
	// Match is how the two sides are compared.
	//
	// exact: both name a value and they must be equal; either naming
	// none is no constraint.
	//
	// band: the room names a range and the joiner brings their own
	// value, which must fall inside it. Asymmetric on purpose — a rank
	// is an attribute of the player rather than a filter they pick, so
	// there is no unset case on their side.
	Match string `default:"exact" help:"exact or band"`
	// Values is the allowed set for an exact axis. Empty accepts any
	// string, which suits a free-form tag and not much else.
	Values []string `help:"allowed values for an exact axis; empty accepts anything"`
	// Low and High bound a band axis, and bound what a room may ask for
	// on it.
	Low  int `default:"0" help:"lowest value a band axis admits"`
	High int `default:"0" help:"highest value a band axis admits"`
}
