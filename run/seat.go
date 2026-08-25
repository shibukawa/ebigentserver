package run

import "github.com/shibukawa/ebigentserver/session"

// SeatKind is who occupies a seat. The session never learns this
// (decision:no-ai-game-mode); it exists so a lobby can draw the roster and
// so data:episode-log can label decisions by controller kind, which is
// what makes a corpus filterable in Phase 7.
//
// There are three, and where the occupant runs is not one of them. That
// is Seat.Local, and it is reported rather than declared: a seating call
// says human or bot, and whether the controller sits in this process
// follows from concept:execution-topology and from which process took
// the host part.
type SeatKind uint8

const (
	// Empty is a declared slot nobody has taken.
	Empty SeatKind = iota
	// Human is a person deciding this seat.
	Human
	// Bot is an agent deciding it — an ordinary concept:agent,
	// including the enemies of a solo game.
	Bot
)

// String names the kind for lobby text and logs.
func (k SeatKind) String() string {
	switch k {
	case Empty:
		return "empty"
	case Human:
		return "human"
	case Bot:
		return "bot"
	default:
		return "invalid"
	}
}

// Seat is one entry of api:roster: a declared concept:player-slot and
// whatever has claimed it.
type Seat struct {
	// Slot is the declared slot this seat fills.
	Slot session.SlotID
	// Kind is who occupies it.
	Kind SeatKind
	// Local reports that the occupant decides inside this process: a
	// person at this keyboard, or an agent running here. It is a
	// result, not a declaration — the same bot is local to whichever
	// process ended up running it, which is the host's or the dedicated
	// server's.
	Local bool
	// ID labels the occupant for the lobby and the episode header — a
	// player name, a bot profile, an enemy kind. It is not an identity
	// credential (rule:identity-token-not-accepted-by-session).
	ID string
	// Ready means this seat has confirmed it wants to start.
	Ready bool
}

// Filled reports whether anything occupies the seat.
func (s Seat) Filled() bool { return s.Kind != Empty }

// LocalHuman reports a person at this machine — the seats this process
// reads devices for.
func (s Seat) LocalHuman() bool { return s.Kind == Human && s.Local }

// LocalBot reports an agent running in this process, which is the one
// kind of seat the match has to drive itself.
func (s Seat) LocalBot() bool { return s.Kind == Bot && s.Local }

// AgentKind is the data:episode-log agent_kind label for this seat. The
// log distinguishes who decided, not where they sat, because that is the
// axis analysis and distillation filter on.
//
// For a bot that is the policy it runs, which is what ID carries: a game
// with three kinds of pursuer records three labels, and a corpus mixing
// them can be split back apart. Without it every enemy reads as "bot"
// and the only way to tell them apart is the seat number, which stops
// working the moment a kind moves seats.
//
// A person is "human" and never their name. Which player it was is
// identity rather than a kind, it is not what analysis filters on, and a
// corpus is not the place to keep it.
func (s Seat) AgentKind() string {
	if s.Kind == Bot && s.ID != "" {
		return s.ID
	}
	return s.Kind.String()
}
