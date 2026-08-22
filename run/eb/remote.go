package eb

import (
	"context"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shibukawa/ebigentserver/run"
)

// remote is the play scene of a guest: the same client, the same intake,
// and no session behind it. The world arrives already committed, so
// there is nothing here to advance and nothing to record — the host owns
// both (term:server-authority).
//
// It is a separate scene from play only because what ends it is
// different. A local match ends when the session says so; a link ends
// when it ends, and the host is the one that knows why.
type remote[S, A, O any] struct {
	app    *app[S, A, O]
	client Client[S, A, O]
	joined run.Joined[S, A, O]
	cancel context.CancelFunc
	done   bool
}

// newRemote starts the link's own goroutine and hands the frame loop a
// scene that only reads and submits.
func newRemote[S, A, O any](a *app[S, A, O], joined run.Joined[S, A, O]) *remote[S, A, O] {
	r := &remote[S, A, O]{app: a, client: a.opts.Client, joined: joined}
	// The sink runs on the link's goroutine, which is where the
	// reconstructed world is safe to read — the same contract a local
	// match's Broadcast has, from the other side of the wire.
	joined.OnWorld(a.opts.Client.Apply)
	ctx, cancel := context.WithCancel(a.ctx)
	r.cancel = cancel
	go func() { _ = joined.Play(ctx) }()
	return r
}

// Update submits this frame's action into the link and notices when the
// link has ended.
func (r *remote[S, A, O]) Update() error {
	if r.joined.Over() {
		return r.ended()
	}
	r.client.Intake(r.joined)
	return nil
}

// Draw renders whatever the last world produced.
func (r *remote[S, A, O]) Draw(screen *ebiten.Image) { r.client.Draw(screen) }

// ended closes the link and leaves the app idle. A guest has no roster
// to return to: the next match is the host's to start, and joining it is
// a fresh run.
func (r *remote[S, A, O]) ended() error {
	if r.done {
		return nil
	}
	r.done = true
	r.cancel()
	_ = r.joined.Close()
	return ebiten.Termination
}
