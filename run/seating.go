package run

import (
	"context"

	"github.com/shibukawa/ebigentserver/session"
)

// Controls is what a play screen needs in order to act: which seats this
// machine plays, and somewhere to put their actions.
//
// A Match satisfies it, and so does a link to somebody else's match.
// That is the whole reason it exists — one play scene serves a host and
// a guest without being told which it is, because from the scene's side
// the difference is only where Submit ends up.
type Controls[A any] interface {
	// LocalSeats is the seats this machine plays.
	LocalSeats() []Seat
	// Submit hands one action to a seat.
	Submit(slot session.SlotID, action A) error
}

// Found is one game a Matchmaking turned up. It carries what somebody
// deciding whether to join needs to read, and nothing about how the link
// is made — that is the preset's business.
type Found struct {
	// Name is what the host called itself.
	Name string
	// Address is how the preset will reach it. It is shown because a
	// player with two networks may care which one answered.
	Address string
	// Players is how many seats were taken when the host last said so.
	Players int
}

// Host is this instance offering its own match to others. The calls are separated because the session does not exist
// when the offer opens: Attach installs the downstream hook into the
// configuration, and Serve is what can only happen once there is a match
// to admit people into.
type Host[W, A, S any] interface {
	// Rebind points the offer at a new roster. A roster belongs to one
	// match (concept:match-lifecycle), so the next match brings a new
	// one, and an offer still filling the finished match's roster
	// would seat arrivals nowhere.
	Rebind(r *Roster[W, A, S])
	// Attach installs the downstream hook before session.New is called.
	Attach(cfg *session.Config[W, A, S])
	// Serve wires the finalized match and admits whoever was waiting.
	Serve(ctx context.Context, m *Match[W, A, S]) error
	// Close stops offering. Admitted links belong to the match.
	Close() error
}

// Guest is this instance playing somebody else's match. There is no
// session here and no simulation: the world arrives already committed,
// and the only thing travelling the other way is data:player-input.
type Guest[W, A, S any] interface {
	Controls[A]
	// OnWorld registers the sink for each reconstructed world. It is
	// called on the link's goroutine, never the frame's, so the sink
	// must copy rather than retain.
	OnWorld(func(session.Tick, *W))
	// Play drives the link until it ends or ctx does.
	Play(ctx context.Context) error
	// Over reports that Play has returned.
	Over() bool
	// Close ends the link.
	Close() error
}

// Matchmaking is how an instance reaches other people. It is three calls
// rather than one because a player is entitled to see who is out there
// and decide, instead of being put into the first match that answered.
//
// A host opens a room and judges nobody after that: what happens when
// somebody arrives is a check against the terms the room already stated,
// which is why none of these calls asks a person for permission.
type Matchmaking[W, A, S any] interface {
	// Discover reports the games this instance can see right now.
	Discover(ctx context.Context) ([]Found, error)
	// Host offers this instance's match to whoever looks.
	Host(ctx context.Context, r *Roster[W, A, S], seed uint64) (Host[W, A, S], error)
	// Match reaches one of the rooms Discover reported. It returns
	// once this instance is in, before the host has started anything.
	Match(ctx context.Context, f Found) (Guest[W, A, S], error)
}

var _ Controls[int] = (*Match[int, int, int])(nil)
