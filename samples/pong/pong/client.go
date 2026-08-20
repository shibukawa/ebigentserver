package pong

import (
	"context"
	"errors"
	"sync"

	"github.com/shibukawa/ebigentserver/samples/pong/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
)

// Client is the loopback client loop for one slot: it reconstructs the
// world from the hub's snapshot/delta stream, projects it into the
// agent's observation, and submits the agent's inputs to the session
// inbox. This is flow:agent-decision-loop with the transport legs running
// in-process; Phase 3b swaps the channel for a network link without
// touching the agent.
//
// The session-side seat is a session.Detached stub; the real agent lives
// here, wholly fed by the state stream.
type Client struct {
	Slot  session.SlotID
	Agent session.Agent[Observation, Input]
	Inbox *session.Inbox[Input]
	Hub   *statesync.Hub[State, msg.PongStateDelta]
	Down  <-chan statesync.Packet
}

// Run consumes packets until the channel closes or ctx is cancelled.
// Call it on its own goroutine; wg tracks completion.
func (c *Client) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	receiver, err := statesync.NewReceiver(Codec())
	if err != nil {
		panic(err) // static misconfiguration, not a runtime condition
	}
	var game Game
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-c.Down:
			if !ok {
				return
			}
			if err := receiver.Apply(pkt); err != nil {
				if errors.Is(err, statesync.ErrResyncNeeded) {
					c.Hub.RequestResync(c.Slot)
					continue
				}
				return
			}
			world, _, ok := receiver.State()
			if !ok {
				continue
			}
			c.Agent.Observe(game.Project(world, c.Slot))
			if in, ok := c.Agent.Decide(ctx); ok {
				c.Inbox.Submit(in)
			}
		}
	}
}
