package netplay

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/seqack"
)

// ackDeadline bounds DelayedPiggyback acks client-side.
const ackDeadline = 50 * time.Millisecond

// ClientConfig assembles the player (or spectator) side.
type ClientConfig[S, A, D, O any] struct {
	// Protocol is the exact wire version.
	Protocol string
	// Tuning must declare the same profile the server runs.
	Tuning session.TuningProfile
	// Codec wires the generated state functions.
	Codec statesync.Codec[S, D]
	// EncodeInput serializes one action for the wire.
	EncodeInput func(dst []byte, a A) []byte
	// Project builds the sight an agent reads
	// (rule:sight-content-owned-by-game — the game's projection,
	// run client-side over the reconstructed world in global-scope
	// phases).
	Project func(world *S, slot session.SlotID) O
}

// Client is one connected participant.
type Client[S, A, D, O any] struct {
	cfg   ClientConfig[S, A, D, O]
	Slot  session.SlotID
	Role  string
	Seed  uint64
	conn  transport.Conn
	layer *seqack.Layer
	recv  *statesync.Receiver[S, D]
}

// ErrSessionLost reports the authoritative side going away: per
// decision:host-loss-ends-session the session is over — the client
// reports loss and returns to whatever came before, never migrates.
var ErrSessionLost = errors.New("netplay: session lost")

// Connect joins: handshake first (version mismatch and bad tickets
// surface as admission.ErrRejected), then the state stream. A spectator
// role automatically uses dedicated acks — with no upstream flow its
// baseline would otherwise never confirm
// (concept:ack-transmission-policy).
func Connect[S, A, D, O any](ctx context.Context, conn transport.Conn, ticket string, cfg ClientConfig[S, A, D, O]) (*Client[S, A, D, O], error) {
	welcome, err := admission.Join(ctx, conn, cfg.Protocol, ticket)
	if err != nil {
		return nil, err
	}
	recv, err := statesync.NewReceiver(cfg.Codec, cfg.Tuning)
	if err != nil {
		return nil, err
	}
	policy := seqack.Policy(cfg.Tuning.AckMode)
	if welcome.Role == RoleSpectator {
		policy = seqack.Dedicated
	}
	return &Client[S, A, D, O]{
		cfg:   cfg,
		Slot:  session.SlotID(welcome.Seat),
		Role:  welcome.Role,
		Seed:  welcome.Seed,
		conn:  conn,
		layer: seqack.New(conn, seqack.Options{Policy: policy, AckDeadline: ackDeadline}),
		recv:  recv,
	}, nil
}

// Run drives the agent from the state stream until the connection closes
// (ErrSessionLost) or ctx cancels (nil). Spectator clients pass an agent
// whose Decide never returns an action — or any agent: its actions are
// simply never sent.
func (c *Client[S, A, D, O]) Run(ctx context.Context, agent session.Agent[O, A]) error {
	agent.Joined(c.Slot)
	for {
		m, err := c.conn.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return ErrSessionLost
		}
		if m.Channel != transport.Unreliable {
			continue
		}
		payload := c.layer.Absorb(m.Payload)
		if payload == nil {
			_ = c.layer.MaybeFlushAck(ctx)
			continue
		}
		pkt, err := statesync.DecodeWire(payload)
		if err != nil {
			continue
		}
		if err := c.recv.Apply(pkt); err != nil {
			if errors.Is(err, statesync.ErrResyncNeeded) {
				body, _ := json.Marshal(control{T: "resync"})
				_ = c.conn.SendReliable(ctx, body)
				continue
			}
			return err
		}
		world, tick, ok := c.recv.State()
		if !ok {
			continue
		}
		agent.Observe(c.cfg.Project(world, c.Slot))
		if in, ok := agent.Decide(ctx); ok && c.Role != RoleSpectator {
			_ = c.layer.SendDatagram(ctx, c.cfg.EncodeInput(nil, in), uint64(tick))
		}
		_ = c.layer.MaybeFlushAck(ctx)
	}
}

// State exposes the newest reconstructed world.
func (c *Client[S, A, D, O]) State() (*S, session.Tick, bool) { return c.recv.State() }

// Stats exposes the measured link.
func (c *Client[S, A, D, O]) Stats() seqack.Stats { return c.layer.Stats() }

// Close tears the connection down.
func (c *Client[S, A, D, O]) Close() error { return c.conn.Close() }
