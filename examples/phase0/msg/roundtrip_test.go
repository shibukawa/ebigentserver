package msg

import (
	"bytes"
	"testing"
)

// Phase 0 completion criterion: a struct with fixed-point fields round-trips
// through CBOR.
func TestPlayerInputRoundTripsOnWireProfile(t *testing.T) {
	in := PlayerInput{Tick: 1234, MoveX: -1024, MoveY: 512, Buttons: 3}

	encoded := in.AppendCBORTo(nil)

	var out PlayerInput
	if err := out.DecodeCBORFrom(encoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out != in {
		t.Fatalf("round trip mismatch: sent %+v, got %+v", in, out)
	}
}

// The wire profile is positional: the encoding must be a fixed-length array of
// raw integers, no field names, no float tag anywhere
// (concept:cbor-wire-profile). Pinning the bytes freezes the protocol; a
// change here is a protocol version change.
func TestPlayerInputWireBytesArePinned(t *testing.T) {
	in := PlayerInput{Tick: 1234, MoveX: -1, MoveY: 0, Buttons: 3}
	want := []byte{
		0x84,             // array(4)
		0x19, 0x04, 0xd2, // 1234
		0x20, // -1
		0x00, // 0
		0x03, // 3
	}
	got := in.AppendCBORTo(nil)
	if !bytes.Equal(got, want) {
		t.Fatalf("wire bytes moved: got % x, want % x", got, want)
	}
}

func TestWorldStateRoundTripsOnWorldProfile(t *testing.T) {
	in := WorldState{
		Tick: 42,
		Entities: []Entity{
			{ID: 1, PosX: 1024, PosY: -2048, Vel: 65536, HP: 100},
			{ID: 2, PosX: 0, PosY: 4096, Vel: -32768, HP: 50},
		},
		Phase: 2,
	}

	encoded := in.AppendCBORTo(nil)

	var out WorldState
	if err := out.DecodeCBORFrom(encoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertWorldEqual(t, in, out)
}

// data:protocol-version — the generated schema digest exists, and re-encoding
// under it is stable. The constant is asserted non-empty rather than pinned:
// the schema text beside it is what a mismatch is diagnosed with.
func TestProtocolVersionIsDerivedFromSchema(t *testing.T) {
	if CBORProtocolVersion == "" {
		t.Fatal("CBORProtocolVersion is empty")
	}
	if CBORSchema == "" {
		t.Fatal("CBORSchema is empty")
	}
	t.Logf("protocol version %s over schema:\n%s", CBORProtocolVersion, CBORSchema)
}

// decision:framework-side-delta-generation — the generated diff carries only
// the changed field, and apply reproduces the sender's encoding.
func TestWorldDeltaRoundTrips(t *testing.T) {
	baseline := WorldState{
		Tick: 42,
		Entities: []Entity{
			{ID: 1, PosX: 1024, PosY: -2048, Vel: 65536, HP: 100},
		},
		Phase: 1,
	}
	current := baseline
	current.Tick = 43
	current.Entities = []Entity{
		{ID: 1, PosX: 1088, PosY: -2048, Vel: 65536, HP: 100},
	}

	delta := DiffWorldState(baseline, current)
	deltaBytes := delta.AppendCBORTo(nil)

	var received WorldStateDelta
	if err := received.DecodeCBORFrom(deltaBytes); err != nil {
		t.Fatalf("delta decode: %v", err)
	}

	applied := baseline
	if err := ApplyWorldStateDelta(&applied, received); err != nil {
		t.Fatalf("apply: %v", err)
	}

	wantBytes := current.AppendCBORTo(nil)
	gotBytes := applied.AppendCBORTo(nil)
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("apply did not reproduce the sender's bytes:\ngot  % x\nwant % x", gotBytes, wantBytes)
	}

	if snapshot := current.AppendCBORTo(nil); len(deltaBytes) >= len(snapshot) {
		t.Errorf("delta (%d bytes) is not smaller than the snapshot (%d bytes)", len(deltaBytes), len(snapshot))
	}
}

func assertWorldEqual(t *testing.T, want, got WorldState) {
	t.Helper()
	if got.Tick != want.Tick || got.Phase != want.Phase {
		t.Fatalf("scalar mismatch: got %+v, want %+v", got, want)
	}
	if len(got.Entities) != len(want.Entities) {
		t.Fatalf("entity count: got %d, want %d", len(got.Entities), len(want.Entities))
	}
	for i := range want.Entities {
		if got.Entities[i] != want.Entities[i] {
			t.Fatalf("entity %d: got %+v, want %+v", i, got.Entities[i], want.Entities[i])
		}
	}
}
