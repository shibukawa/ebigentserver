// Package msg is the Phase 0 proving ground: a wire-profile message and a
// world-profile state whose fixed-point fields round-trip through the
// generated CBOR codecs (concept:cbor-wire-profile, concept:cbor-world-profile).
//
// The fixed-point representation follows decision:fixed-point-numeric-representation:
// a value times a per-field scale, stored as a sized integer. The scale is the
// type's — one declared type per scale — so the generator never sees a scale
// and never converts one.
package msg

import (
	"github.com/shibukawa/fixmath"
	"github.com/shibukawa/tinybind-go/cborbind"
	"github.com/shibukawa/tinygodriver/encoding/cbor"
)

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -openapi=false

// Fixed1024 is a fixed-point value at 1/1024 (shift 10).
type Fixed1024 int64

// AppendCBORTo encodes the raw scaled integer; the scale is declared by the
// type and never travels on the wire (rule:fixed-point-on-wire).
func (f Fixed1024) AppendCBORTo(dst []byte) []byte { return cbor.AppendInt(dst, int64(f)) }

// F64 converts the wire value into the canonical compute format
// (api:fixed-point-math). Exact: shift 10 <= 32.
func (f Fixed1024) F64() fixmath.F64 { return fixmath.FromScaled(int64(f), 10) }

// Fixed1024FromF64 quantizes a compute value onto the wire scale, rounding
// half away from zero (fixmath FR-7).
func Fixed1024FromF64(v fixmath.F64) Fixed1024 { return Fixed1024(fixmath.ToScaled(v, 10)) }

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

// Fixed65536 is a fixed-point value at 1/65536 (shift 16), for quantities
// needing more fractional precision than position does.
type Fixed65536 int64

// AppendCBORTo encodes the raw scaled integer.
func (f Fixed65536) AppendCBORTo(dst []byte) []byte { return cbor.AppendInt(dst, int64(f)) }

// F64 converts the wire value into the canonical compute format. Exact.
func (f Fixed65536) F64() fixmath.F64 { return fixmath.FromScaled(int64(f), 16) }

// Fixed65536FromF64 quantizes a compute value onto the wire scale, rounding
// half away from zero.
func Fixed65536FromF64(v fixmath.F64) Fixed65536 { return Fixed65536(fixmath.ToScaled(v, 16)) }

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

// PlayerInput is a per-tick message on the wire profile: fixed field order,
// no field names, sized integers only (data:player-input).
type PlayerInput struct {
	Tick    uint32
	MoveX   Fixed1024
	MoveY   Fixed1024
	Buttons uint16
}

var _ = cborbind.GenerateWireCodec[PlayerInput]()

// Entity is one identified world object; the identity lets the generated
// delta diff a collection element by element.
type Entity struct {
	ID   uint32     `cbor:"id,key=1,identity"`
	PosX Fixed1024  `cbor:"x,key=2"`
	PosY Fixed1024  `cbor:"y,key=3"`
	Vel  Fixed65536 `cbor:"vel,key=4"`
	HP   int32      `cbor:"hp,key=5"`
}

// WorldState is the world-profile root (concept:world-state,
// decision:go-struct-world-state): an evolvable map encoding that a snapshot
// can outlive the version that wrote it.
type WorldState struct {
	Tick     uint64   `cbor:"tick,key=1"`
	Entities []Entity `cbor:"entities,key=2"`
	Phase    uint8    `cbor:"phase,key=3"`
}

var _ = cborbind.GenerateWorldDelta[WorldState]()
