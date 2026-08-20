// Package msg declares pong's wire types: the world state on the world
// profile (snapshot + generated delta for decision:framework-side-delta-
// generation) and the per-tick input on the wire profile
// (data:player-input). Physics computes in fixmath F64 and quantizes onto
// the declared scales at the state boundary.
package msg

import (
	"github.com/shibukawa/fixmath"
	"github.com/shibukawa/tinybind-go/cborbind"
	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

// Fixed1024 is a fixed-point value at 1/1024 (shift 10): positions.
type Fixed1024 int64

// AppendCBORTo encodes the raw scaled integer (rule:fixed-point-on-wire).
func (f Fixed1024) AppendCBORTo(dst []byte) []byte { return cbor.AppendInt(dst, int64(f)) }

// F64 converts to the compute format; exact for shift 10.
func (f Fixed1024) F64() fixmath.F64 { return fixmath.FromScaled(int64(f), 10) }

// Fixed1024FromF64 quantizes a compute value onto the wire scale.
func Fixed1024FromF64(v fixmath.F64) Fixed1024 { return Fixed1024(v.ToScaled(10, 64)) }

// DecodeCBORFrom reads one scaled integer.
func (f *Fixed1024) DecodeCBORFrom(data []byte) error {
	r := cbor.ReaderOver(data, cbor.DecoderOptions{})
	v, err := r.ReadInt()
	if err != nil {
		return err
	}
	if !r.Done() {
		return cbor.ErrExtraneousData
	}
	*f = Fixed1024(v)
	return nil
}

// Fixed65536 is a fixed-point value at 1/65536 (shift 16): velocities.
type Fixed65536 int64

// AppendCBORTo encodes the raw scaled integer.
func (f Fixed65536) AppendCBORTo(dst []byte) []byte { return cbor.AppendInt(dst, int64(f)) }

// F64 converts to the compute format; exact.
func (f Fixed65536) F64() fixmath.F64 { return fixmath.FromScaled(int64(f), 16) }

// Fixed65536FromF64 quantizes a compute value onto the wire scale.
func Fixed65536FromF64(v fixmath.F64) Fixed65536 { return Fixed65536(v.ToScaled(16, 64)) }

// DecodeCBORFrom reads one scaled integer.
func (f *Fixed65536) DecodeCBORFrom(data []byte) error {
	r := cbor.ReaderOver(data, cbor.DecoderOptions{})
	v, err := r.ReadInt()
	if err != nil {
		return err
	}
	if !r.Done() {
		return cbor.ErrExtraneousData
	}
	*f = Fixed65536(v)
	return nil
}

// PaddleInput is data:player-input on the wire profile: fixed field
// order, sized integers, no field names.
type PaddleInput struct {
	// Tick is the client's tick estimate; the authoritative session
	// decides the applied tick (decision:input-timing-owned-by-sync-mode).
	Tick uint32
	// MoveY is -1 (up), 0, or +1 (down).
	MoveY int8
	// Buttons is reserved bit flags.
	Buttons uint8
}

var _ = cborbind.GenerateWireCodec[PaddleInput]()

// PongState is the authoritative world (concept:world-state) on the world
// profile, diffed by the generated delta for data:state-delta.
type PongState struct {
	Tick   uint64     `cbor:"tick,key=1"`
	BallX  Fixed1024  `cbor:"bx,key=2"`
	BallY  Fixed1024  `cbor:"by,key=3"`
	VelX   Fixed65536 `cbor:"vx,key=4"`
	VelY   Fixed65536 `cbor:"vy,key=5"`
	LeftY  Fixed1024  `cbor:"ly,key=6"`
	RightY Fixed1024  `cbor:"ry,key=7"`
	// ScoreL and ScoreR count points; first to WinScore ends the game.
	ScoreL uint8 `cbor:"sl,key=8"`
	ScoreR uint8 `cbor:"sr,key=9"`
	// Winner is the winning slot, 0 while playing.
	Winner uint16 `cbor:"win,key=10"`
	Over   bool   `cbor:"over,key=11"`
}

var _ = cborbind.GenerateWorldDelta[PongState]()
