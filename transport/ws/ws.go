// Package ws is the WebSocket implementation of api:transport-interface:
// the reliable-ordered fallback transport
// (rule:transport-selected-by-capability — code asks for capabilities,
// and this one offers a reliable stream, browser reachability, and no
// datagrams). SendUnreliable is carried over the stream: delivery order
// and arrival are guaranteed even though the caller did not ask for them,
// which api:sequence-ack-layer tolerates by design.
//
// A one-byte channel prefix distinguishes the two logical channels inside
// the single WebSocket stream.
package ws

import (
	"context"
	"net/http"
	"sync"

	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/tinygodriver/websocket"
)

const (
	prefixReliable   = 0x01
	prefixUnreliable = 0x02
)

// Conn adapts one WebSocket connection.
type Conn struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
	readMu  sync.Mutex
	closed  sync.Once
	done    chan struct{}
}

var _ transport.Conn = (*Conn)(nil)

func wrap(c *websocket.Conn) *Conn {
	return &Conn{ws: c, done: make(chan struct{})}
}

// Capability declares the reliable-only profile.
func (c *Conn) Capability() transport.Capability {
	return transport.Capability{ReliableStream: true, Browser: true}
}

func (c *Conn) send(ctx context.Context, prefix byte, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-c.done:
		return transport.ErrClosed
	default:
	}
	frame := make([]byte, 0, len(payload)+1)
	frame = append(frame, prefix)
	frame = append(frame, payload...)
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.ws.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		return transport.ErrClosed
	}
	return nil
}

// SendReliable sends on the ordered stream.
func (c *Conn) SendReliable(ctx context.Context, payload []byte) error {
	return c.send(ctx, prefixReliable, payload)
}

// SendUnreliable sends over the same stream — reliable in practice, which
// the interface permits (datagrams may but need not be dropped).
func (c *Conn) SendUnreliable(ctx context.Context, payload []byte) error {
	return c.send(ctx, prefixUnreliable, payload)
}

// Receive blocks for the next message.
func (c *Conn) Receive(ctx context.Context) (transport.Message, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.ws.SetReadDeadline(deadline)
	}
	for {
		select {
		case <-c.done:
			return transport.Message{}, transport.ErrClosed
		default:
		}
		kind, data, err := c.ws.ReadMessage()
		if err != nil {
			return transport.Message{}, transport.ErrClosed
		}
		if kind != websocket.BinaryMessage || len(data) < 1 {
			continue // dropped: not ours
		}
		ch := transport.Reliable
		if data[0] == prefixUnreliable {
			ch = transport.Unreliable
		} else if data[0] != prefixReliable {
			continue
		}
		return transport.Message{Channel: ch, Payload: data[1:]}, nil
	}
}

// Close tears the connection down; idempotent.
func (c *Conn) Close() error {
	var err error
	c.closed.Do(func() {
		close(c.done)
		err = c.ws.Close()
	})
	return err
}

// Dial connects to a WebSocket endpoint (ws:// or wss://).
func Dial(ctx context.Context, url string) (*Conn, error) {
	d := *websocket.DefaultDialer
	conn, resp, err := d.DialContext(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	return wrap(conn), nil
}

// Upgrader accepts WebSocket connections inside an http.Handler.
type Upgrader struct {
	u websocket.Upgrader
}

// NewUpgrader builds an accept-all upgrader; origin policy belongs to the
// surrounding HTTP stack.
func NewUpgrader() *Upgrader {
	return &Upgrader{u: websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}}
}

// Upgrade turns one HTTP request into a transport connection.
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	c, err := u.u.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return wrap(c), nil
}
