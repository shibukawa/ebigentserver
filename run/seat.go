package run

import "github.com/shibukawa/ebigentserver/session"

// SeatKind is what occupies a seat. The session never learns this
// (decision:no-ai-game-mode); it exists so a lobby can draw the roster and
// so data:episode-log can label decisions by controller kind, which is
// what makes a corpus filterable in Phase 7.
type SeatKind uint8

const (
	// Empty is a declared slot nobody has taken.
	Empty SeatKind = iota
	// LocalHuman is a person at this machine.
	LocalHuman
	// RemoteHuman is a person on the far side of a link.
	RemoteHuman
	// LocalBot is a controller running in this process — an ordinary
	// agent, including the enemies of a solo game.
	LocalBot
	// RemoteBot is a controller running in another process.
	RemoteBot
)

// String names the kind for lobby text and logs.
func (k SeatKind) String() string {
	switch k {
	case Empty:
		return "empty"
	case LocalHuman:
		return "local_human"
	case RemoteHuman:
		return "remote_human"
	case LocalBot:
		return "local_bot"
	case RemoteBot:
		return "remote_bot"
	default:
		return "invalid"
	}
}

// Local reports whether the controller runs in this process.
func (k SeatKind) Local() bool { return k == LocalHuman || k == LocalBot }

// Human reports whether a person decides this seat.
func (k SeatKind) Human() bool { return k == LocalHuman || k == RemoteHuman }

// AgentKind is the data:episode-log agent_kind label for this seat. The
// log distinguishes who decided, not where they sat, because that is the
// axis analysis and distillation filter on.
func (k SeatKind) AgentKind() string {
	switch {
	case k.Human():
		return "human"
	case k == LocalBot || k == RemoteBot:
		return "bot"
	default:
		return "empty"
	}
}

// Seat is one entry of api:roster: a declared concept:player-slot and
// whatever has claimed it.
type Seat struct {
	// Slot is the declared slot this seat fills.
	Slot session.SlotID
	// Kind is what occupies it.
	Kind SeatKind
	// ID labels the occupant for the lobby and the episode header — a
	// player name, a bot profile, an enemy kind. It is not an identity
	// credential (rule:identity-token-not-accepted-by-session).
	ID string
	// Ready means this seat has confirmed it wants to start.
	Ready bool
}

// Filled reports whether anything occupies the seat.
func (s Seat) Filled() bool { return s.Kind != Empty }
