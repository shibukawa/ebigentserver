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

// Hosting is a networking preset offering this instance's match to
// others. The two calls are separated because the session does not exist
// when the offer opens: Attach installs the downstream hook into the
// configuration, and Serve is what can only happen once there is a match
// to admit people into.
type Hosting[S, A, O any] interface {
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

// Networking decides which part this instance plays. A preset that finds
// a host joins it and returns Joined; one that finds none offers its own
// match and returns Hosting. Exactly one of the two is non-nil.
type Networking[S, A, O any] interface {
	Begin(ctx context.Context, r *Roster[S, A, O], seed uint64) (Hosting[S, A, O], Joined[S, A, O], error)
}

var _ Seating[int] = (*Match[int, int, int])(nil)
