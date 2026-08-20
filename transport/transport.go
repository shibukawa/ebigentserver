// Package transport defines api:transport-interface: the abstraction that
// carries agent and session messages over any supported protocol. Code
// selects a transport by its declared capability set, never by protocol
// name (rule:transport-selected-by-capability), so browser and native
// builds share selection logic.
//
// Layering (bottom up): a Conn implementation (pipe, websocket,
// webtransport, ...) → api:message-framing for large reliable payloads →
// api:sequence-ack-layer for the unreliable state stream → the session.
package transport

import (
	"context"
	"errors"
)

// Capability is concept:transport-capability.
type Capability struct {
	// ReliableStream: ordered, guaranteed delivery.
	ReliableStream bool
	// UnreliableDatagram: may drop and reorder.
	UnreliableDatagram bool
	// PeerToPeer: can connect without a server in the middle.
	PeerToPeer bool
	// Browser: reachable from a browser build.
	Browser bool
}

// Channel identifies which delivery class a message used.
type Channel uint8

const (
	// Reliable is the ordered stream.
	Reliable Channel = iota + 1
	// Unreliable is the datagram channel.
	Unreliable
)

// Message is one received payload with its delivery class. The payload is
// ownership-safe: the transport never reuses it after delivery.
type Message struct {
	Channel Channel
	Payload []byte
}

// Errors every implementation maps onto.
var (
	ErrClosed       = errors.New("transport: connection closed")
	ErrTooLarge     = errors.New("transport: message exceeds transport limit")
	ErrBackpressure = errors.New("transport: send queue full")
	ErrUnsupported  = errors.New("transport: channel not supported")
)

// Conn is one established connection. Implementations must allow
// concurrent senders (order preserved only within the reliable channel)
// and one receiver at a time; Close is idempotent and unblocks Receive.
//
// The framework never retries an accepted action through a Conn;
// reconnecting is a fresh flow:session-admission
// (decision:no-mid-session-reconnect).
type Conn interface {
	// SendReliable queues one message on the ordered stream.
	SendReliable(ctx context.Context, payload []byte) error
	// SendUnreliable queues one datagram; it may be dropped or
	// reordered in flight. A transport without datagram support
	// returns ErrUnsupported — callers decide whether to fall back to
	// the reliable stream (rule:transport-selected-by-capability).
	SendUnreliable(ctx context.Context, payload []byte) error
	// Receive blocks for the next message until ctx or close.
	Receive(ctx context.Context) (Message, error)
	// Close tears the connection down; idempotent.
	Close() error
	// Capability declares what this transport offers.
	Capability() Capability
}
