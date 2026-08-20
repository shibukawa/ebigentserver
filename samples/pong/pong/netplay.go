package pong

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/samples/pong/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/seqack"
)

// This file wires pong across a real transport.Conn: state packets ride
// datagrams under api:sequence-ack-layer, inputs ride datagrams the other
// way, and a tiny reliable control envelope carries resync requests. The
// same code runs over the fault-injecting pipe and over WebSocket —
// selection is by capability, not name.

// control is the reliable-channel envelope.
type control struct {
	T string `json:"t"`
}

// ackDeadline bounds DelayedPiggyback acks client-side.
const ackDeadline = 50 * time.Millisecond

func policyFor(tuning session.TuningProfile) seqack.Policy {
	switch tuning.AckMode {
	case 1:
		return seqack.Dedicated
	case 2:
		return seqack.DelayedPiggyback
	default:
		return seqack.PiggybackOnly
	}
}

// RemotePeer is the server side of one admitted connection: it turns the
// session's broadcast into sequenced state datagrams for this receiver
// and the receiver's datagrams into inbox submissions.
type RemotePeer struct {
	Slot session.SlotID

	conn  transport.Conn
	layer *seqack.Layer
	inbox *session.Inbox[Input]

	mu     sync.Mutex
	sender *statesync.Sender[State, msg.PongStateDelta]
}

// NewRemotePeer builds the peer after admission seated the connection.
func NewRemotePeer(conn transport.Conn, slot session.SlotID, inbox *session.Inbox[Input], tuning session.TuningProfile) (*RemotePeer, error) {
	snd, err := statesync.NewSender(Codec(), tuning)
	if err != nil {
		return nil, err
	}
	return &RemotePeer{
		Slot:   slot,
		conn:   conn,
		layer:  seqack.New(conn, seqack.Options{Policy: seqack.PiggybackOnly}),
		inbox:  inbox,
		sender: snd,
	}, nil
}

// SendState encodes and sends one committed world version.
func (p *RemotePeer) SendState(ctx context.Context, tick session.Tick, world *State) {
	p.mu.Lock()
	if conf, ok := p.layer.Confirmed(); ok {
		p.sender.Confirm(session.Tick(conf))
	}
	pkt := p.sender.Send(tick, world)
	p.mu.Unlock()
	_ = p.layer.SendDatagram(ctx, pkt.AppendWire(nil), uint64(tick))
}

// Run consumes the connection until it closes: input datagrams into the
// inbox, resync requests into the sender. Run it on its own goroutine.
func (p *RemotePeer) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		m, err := p.conn.Receive(ctx)
		if err != nil {
			return
		}
		switch m.Channel {
		case transport.Unreliable:
			payload := p.layer.Absorb(m.Payload)
			if payload == nil {
				continue
			}
			var in msg.PaddleInput
			if err := in.DecodeCBORFrom(payload); err != nil {
				continue // malformed input: dropped before the session
			}
			p.inbox.Submit(in)
		case transport.Reliable:
			var c control
			if json.Unmarshal(m.Payload, &c) == nil && c.T == "resync" {
				p.mu.Lock()
				p.sender.ResyncRequested()
				p.mu.Unlock()
			}
		}
	}
}

// PeerSet fans the session broadcast out to every admitted peer; it
// matches session.Config.Broadcast.
type PeerSet struct {
	mu    sync.Mutex
	peers []*RemotePeer
	ctx   context.Context
}

// NewPeerSet builds the set; ctx bounds the sends.
func NewPeerSet(ctx context.Context) *PeerSet { return &PeerSet{ctx: ctx} }

// Add registers an admitted peer.
func (ps *PeerSet) Add(p *RemotePeer) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.peers = append(ps.peers, p)
}

// Broadcast sends the committed world to every peer.
func (ps *PeerSet) Broadcast(tick session.Tick, world *State) {
	ps.mu.Lock()
	peers := append([]*RemotePeer(nil), ps.peers...)
	ps.mu.Unlock()
	for _, p := range peers {
		p.SendState(ps.ctx, tick, world)
	}
}

// AdmitRemote runs the server handshake on a fresh connection and seats
// it: version check, local ticket verification, seat lookup, peer
// creation (flow:session-admission steps connect through seat).
func AdmitRemote[SlotSource interface {
	Inbox(session.SlotID) (*session.Inbox[Input], error)
}](ctx context.Context, conn transport.Conn, src SlotSource, verifier *admission.Verifier, seed uint64, tuning session.TuningProfile) (*RemotePeer, error) {
	claims, err := admission.Accept(ctx, conn, msg.CBORProtocolVersion, verifier, seed)
	if err != nil {
		return nil, err
	}
	slot := session.SlotID(claims.Seat)
	inbox, err := src.Inbox(slot)
	if err != nil {
		return nil, fmt.Errorf("pong: ticket names unknown seat %d: %w", claims.Seat, err)
	}
	return NewRemotePeer(conn, slot, inbox, tuning)
}

// NetClient is the player side over a real connection.
type NetClient struct {
	Slot  session.SlotID
	Seed  uint64
	conn  transport.Conn
	layer *seqack.Layer
	recv  *statesync.Receiver[State, msg.PongStateDelta]
}

// Connect joins a session: handshake first (a protocol version mismatch
// or bad ticket surfaces here as admission.ErrRejected), then the state
// stream.
func Connect(ctx context.Context, conn transport.Conn, ticket string, tuning session.TuningProfile) (*NetClient, error) {
	welcome, err := admission.Join(ctx, conn, msg.CBORProtocolVersion, ticket)
	if err != nil {
		return nil, err
	}
	recv, err := statesync.NewReceiver(Codec(), tuning)
	if err != nil {
		return nil, err
	}
	return &NetClient{
		Slot:  session.SlotID(welcome.Seat),
		Seed:  welcome.Seed,
		conn:  conn,
		layer: seqack.New(conn, seqack.Options{Policy: policyFor(tuning), AckDeadline: ackDeadline}),
		recv:  recv,
	}, nil
}

// Run drives the agent from the network state stream until the
// connection closes or ctx cancels: reconstructed world → Observe →
// Decide → input datagram. Acks flush per the declared policy so a
// silent agent still confirms baselines.
func (c *NetClient) Run(ctx context.Context, agent session.Agent[Observation, Input]) error {
	var game Game
	agent.Joined(c.Slot)
	for {
		m, err := c.conn.Receive(ctx)
		if err != nil {
			if errors.Is(err, transport.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			return err
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
		world, _, ok := c.recv.State()
		if !ok {
			continue
		}
		agent.Observe(game.Project(world, c.Slot))
		if in, ok := agent.Decide(ctx); ok {
			buf := in.AppendCBORTo(nil)
			_ = c.layer.SendDatagram(ctx, buf, uint64(world.Tick))
		}
		_ = c.layer.MaybeFlushAck(ctx)
	}
}

// State exposes the newest reconstructed world (rendering).
func (c *NetClient) State() (*State, session.Tick, bool) { return c.recv.State() }

// Stats exposes the measured link (concept:delta-baseline-policy's
// adaptive inputs, and silence detection material).
func (c *NetClient) Stats() seqack.Stats { return c.layer.Stats() }
