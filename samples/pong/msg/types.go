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

// PongState is the authoritative world (concept:world-state) on the world
// profile, diffed by the generated delta for data:state-delta.
type PongState struct {
	Tick   uint64     `json:"tick"`
	BallX  Fixed1024  `json:"bx"`
	BallY  Fixed1024  `json:"by"`
	VelX   Fixed65536 `json:"vx"`
	VelY   Fixed65536 `json:"vy"`
	LeftY  Fixed1024  `json:"ly"`
	RightY Fixed1024  `json:"ry"`
	// ScoreL and ScoreR count points; first to WinScore ends the game.
	ScoreL uint8 `json:"sl"`
	ScoreR uint8 `json:"sr"`
	// Winner is the winning slot, 0 while playing.
	Winner uint16 `json:"win"`
	Over   bool   `json:"over"`
}

// The calls below are what ask the generator for each codec: there is no
// declaration to write any more, and naming an entry point is the ask
// (requirement:cborbind-migration).
//
// Which container a type uses is a contract rather than a preference. An
// input is an array — positional, no field names on the wire, and both
// ends rebuilt together — which is concept:cbor-wire-profile. A world
// state is a map, so a decoder can skip a key it does not know and the two
// ends may ship apart, which is concept:cbor-world-profile.


// AppendPaddleInput writes one paddleinput in the array shape.
func AppendPaddleInput(dst []byte, v PaddleInput) []byte { return cborbind.AppendCBORInArrayTo(dst, v) }

// DecodePaddleInput reads one paddleinput.
func DecodePaddleInput(data []byte) (PaddleInput, error) { return cborbind.DecodeCBORInArrayFrom[PaddleInput](data) }


// AppendPongState writes one pongstate in the map shape.
func AppendPongState(dst []byte, v PongState) []byte { return cborbind.AppendCBORInMapTo(dst, v) }

// DecodePongState reads one pongstate.
func DecodePongState(data []byte) (PongState, error) { return cborbind.DecodeCBORInMapFrom[PongState](data) }
