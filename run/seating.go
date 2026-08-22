package run

import (
	"context"

	"github.com/shibukawa/ebigentserver/session"
)

// Seating is what a play scene needs in order to act: which seats belong
// to this machine, and somewhere to put their actions.
//
// A Match satisfies it, and so does a link to somebody else's match.
// That is the whole reason it exists — one play scene serves a host and
// a guest without being told which it is, because from the scene's side
// the difference is only where Submit ends up.
type Seating[A any] interface {
	// LocalSeats is the seats this machine plays.
	LocalSeats() []Seat
	// Submit hands one action to a seat.
	Submit(slot session.SlotID, action A) error
}

// Found is one game a Networking turned up. It carries what somebody
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

// Hosting is a networking preset offering this instance's match to
// others. The calls are separated because the session does not exist
// when the offer opens: Attach installs the downstream hook into the
// configuration, and Serve is what can only happen once there is a match
// to admit people into.
type Hosting[S, A, O any] interface {
	// Rebind points the offer at a new roster. A roster belongs to one
	// match (concept:match-lifecycle), so the next match brings a new
	// one, and an offer still filling the finished match's roster
	// would seat arrivals nowhere.
	Rebind(r *Roster[S, A, O])
	// Attach installs the downstream hook before session.New is called.
	Attach(cfg *session.Config[S, A, O])
	// Serve wires the finalized match and admits whoever was waiting.
	Serve(ctx context.Context, m *Match[S, A, O]) error
	// Close stops offering. Admitted links belong to the match.
	Close() error
}

// Joined is this instance playing somebody else's match. There is no
// session here and no simulation: the world arrives already committed,
// and the only thing travelling the other way is data:player-input.
type Joined[S, A, O any] interface {
	Seating[A]
	// OnWorld registers the sink for each reconstructed world. It is
	// called on the link's goroutine, never the frame's, so the sink
	// must copy rather than retain.
	OnWorld(func(session.Tick, *S))
	// Play drives the link until it ends or ctx does.
	Play(ctx context.Context) error
	// Over reports that Play has returned.
	Over() bool
	// Close ends the link.
	Close() error
}

// Networking is how an instance reaches other people. It is three calls
// rather than one because a player is entitled to see who is out there
// and decide, instead of being put into the first match that answered.
type Networking[S, A, O any] interface {
	// Discover reports the games this instance can see right now.
	Discover(ctx context.Context) ([]Found, error)
	// Host offers this instance's match to whoever looks.
	Host(ctx context.Context, r *Roster[S, A, O], seed uint64) (Hosting[S, A, O], error)
	// Join takes a seat on one of the games Discover reported. It
	// returns once the seat is granted, before the host has started
	// anything.
	Join(ctx context.Context, f Found) (Joined[S, A, O], error)
}

var _ Seating[int] = (*Match[int, int, int])(nil)
