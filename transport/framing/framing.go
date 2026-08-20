// Package framing implements api:message-framing: chunking, reassembly,
// and bounds for transports whose reliable channel caps message size
// (WebRTC data channels being the motivating case — a data:snapshot
// easily exceeds their limit). Malformed frames, out-of-range indexes,
// duplicates, and oversized sets are dropped here, before the session
// layer ever sees them.
//
// Frame layout (all integers big-endian):
//
//	byte 0     : 0xEB marker
//	byte 1     : version (1)
//	bytes 2-5  : message id
//	bytes 6-7  : chunk index
//	bytes 8-9  : chunk count
//	bytes 10.. : chunk payload
//
// Position: on the reliable channel between api:transport-interface and
// the layers above. The per-tick data:player-input path does not pass
// through here.
package framing

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/shibukawa/ebigentserver/transport"
)

const (
	marker  = 0xEB
	version = 1
	header  = 10
)

// Limits bounds the framer. Every field is required
// (decision:no-framework-tuning-defaults spirit: the game sizes these
// from data:session-tuning-profile and data:runtime-resource-budget).
type Limits struct {
	// ChunkSize is the payload bytes per frame; about 12KB stays below
	// the smallest practical data-channel message limit.
	ChunkSize int32
	// MaxMessageSize bounds one reassembled message, sized from
	// max_snapshot_size.
	MaxMessageSize int32
	// MaxPending bounds concurrent partial reassemblies, so a partial
	// flood cannot exhaust memory.
	MaxPending int32
}

// Validate checks the limits.
func (l Limits) Validate() error {
	if l.ChunkSize <= 0 || l.MaxMessageSize <= 0 || l.MaxPending <= 0 {
		return errors.New("framing: every limit must be declared and positive")
	}
	if l.MaxMessageSize < l.ChunkSize {
		return errors.New("framing: MaxMessageSize below ChunkSize")
	}
	return nil
}

// ErrTooLarge rejects a message over MaxMessageSize before any send.
var ErrTooLarge = errors.New("framing: message exceeds MaxMessageSize")

// Framer chunks outgoing reliable messages and reassembles incoming ones.
// It is not safe for concurrent Send calls (serialize sends through one
// queue, as the concept's backpressure section says); Absorb is called
// from the single receive loop.
type Framer struct {
	limits Limits
	conn   transport.Conn
	nextID uint32
	// partial reassemblies by message id.
	pending map[uint32]*assembly
}

type assembly struct {
	count  uint16
	got    uint16
	chunks [][]byte
	size   int32
}

// New builds a framer over a connection's reliable channel.
func New(conn transport.Conn, limits Limits) (*Framer, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	return &Framer{limits: limits, conn: conn, pending: map[uint32]*assembly{}}, nil
}

// Send chunks one message onto the reliable channel.
func (f *Framer) Send(ctx context.Context, payload []byte) error {
	if int32(len(payload)) > f.limits.MaxMessageSize {
		return fmt.Errorf("%w: %d bytes", ErrTooLarge, len(payload))
	}
	id := f.nextID
	f.nextID++
	chunk := int(f.limits.ChunkSize)
	count := (len(payload) + chunk - 1) / chunk
	if count == 0 {
		count = 1
	}
	if count > 0xFFFF {
		return fmt.Errorf("%w: %d chunks", ErrTooLarge, count)
	}
	frame := make([]byte, 0, header+chunk)
	for i := 0; i < count; i++ {
		lo, hi := i*chunk, min((i+1)*chunk, len(payload))
		frame = frame[:header]
		frame[0], frame[1] = marker, version
		binary.BigEndian.PutUint32(frame[2:6], id)
		binary.BigEndian.PutUint16(frame[6:8], uint16(i))
		binary.BigEndian.PutUint16(frame[8:10], uint16(count))
		frame = append(frame, payload[lo:hi]...)
		if err := f.conn.SendReliable(ctx, frame); err != nil {
			return err
		}
	}
	return nil
}

// Absorb consumes one received reliable payload. It returns the
// reassembled message once complete, or (nil, nil) while parts are
// outstanding or the frame was dropped as malformed — dropping, not
// erroring, is the contract: garbage from the wire must not kill the
// receive loop, and policy:realtime-abuse-protection counts drops.
func (f *Framer) Absorb(frame []byte) ([]byte, error) {
	if len(frame) < header || frame[0] != marker || frame[1] != version {
		return nil, nil // malformed: dropped
	}
	id := binary.BigEndian.Uint32(frame[2:6])
	index := binary.BigEndian.Uint16(frame[6:8])
	count := binary.BigEndian.Uint16(frame[8:10])
	payload := frame[header:]
	if count == 0 || index >= count {
		return nil, nil // out of range: dropped
	}
	if int32(count)*f.limits.ChunkSize > f.limits.MaxMessageSize+f.limits.ChunkSize {
		return nil, nil // oversized set: dropped before allocation
	}
	as, ok := f.pending[id]
	if !ok {
		if count == 1 {
			return clone(payload), nil // fast path: single-chunk message
		}
		if int32(len(f.pending)) >= f.limits.MaxPending {
			return nil, nil // partial flood: dropped
		}
		as = &assembly{count: count, chunks: make([][]byte, count)}
		f.pending[id] = as
	}
	if as.count != count || as.chunks[index] != nil {
		return nil, nil // inconsistent or duplicate: dropped
	}
	as.size += int32(len(payload))
	if as.size > f.limits.MaxMessageSize {
		delete(f.pending, id)
		return nil, nil // oversized: dropped
	}
	as.chunks[index] = clone(payload)
	as.got++
	if as.got < as.count {
		return nil, nil
	}
	delete(f.pending, id)
	msg := make([]byte, 0, as.size)
	for _, c := range as.chunks {
		msg = append(msg, c...)
	}
	return msg, nil
}

func clone(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
