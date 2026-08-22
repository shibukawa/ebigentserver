package codegen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/codegen"
)

// driverRequire is the tinygodriver line this repository already pins, so
// a generated package under test links the same encoder the real ones do.
func driverRequire(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.Lines(string(body)) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "github.com/shibukawa/tinygodriver ") {
			return line
		}
	}
	t.Fatal("the repository does not require tinygodriver")
	return ""
}

// TestGeneratedDeltaRoundTrips is the test that matters: it generates a
// delta, compiles it, and runs a real diff → encode → decode → patch over
// every field shape a game uses. Reading the emitted source proves it
// parses; only running it proves a delta puts a world back.
func TestGeneratedDeltaRoundTrips(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs a generated package")
	}
	dir := t.TempDir()
	write(t, dir, "go.mod", "module probe\n\ngo 1.24\n\nrequire "+driverRequire(t)+"\n")

	// One fixture carrying every kind the reader classifies, so the
	// emitter is exercised on all of them at once.
	write(t, dir, "types.go", `package probe

import "github.com/shibukawa/tinygodriver/encoding/cbor"

// Scaled is a named type over a signed integer — the shape the
// fixed-point scales take.
type Scaled int64

type Mote struct {
	ID uint16
	HP int8
}

func (v Mote) AppendCBORTo(dst []byte) []byte {
	dst = cbor.AppendArrayHeader(dst, 2)
	dst = cbor.AppendUint(dst, uint64(v.ID))
	return cbor.AppendInt(dst, int64(v.HP))
}

func (v *Mote) DecodeCBORFrom(data []byte) error {
	r, err := cbor.NewReader(data, cbor.DecoderOptions{MaxContainerItems: 64, MaxInputBytes: 1 << 20})
	if err != nil {
		return err
	}
	if _, _, err := r.ReadArrayHeader(); err != nil {
		return err
	}
	id, err := r.ReadUint64()
	if err != nil {
		return err
	}
	hp, err := r.ReadInt64()
	if err != nil {
		return err
	}
	v.ID, v.HP = uint16(id), int8(hp)
	return nil
}

type World struct {
	Tick   uint64
	Depth  Scaled
	Over   bool
	Cells  []uint8
	Motes  []Mote
}
`)

	// The reader type-checks, so the module has to resolve its imports
	// before it can be read at all.
	if out, err := runIn(dir, "go", "mod", "tidy"); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}

	pkg, structs, err := codegen.Read(dir, []string{"World"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	src, err := codegen.Emit(pkg, structs)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	write(t, dir, "delta_gen.go", string(src))

	// The exercise runs inside the generated package, since that is the
	// only place the emitted identifiers are in scope.
	write(t, dir, "roundtrip_test.go", `package probe

import "testing"

func same(a, b World) bool {
	if a.Tick != b.Tick || a.Depth != b.Depth || a.Over != b.Over {
		return false
	}
	if string(a.Cells) != string(b.Cells) || len(a.Motes) != len(b.Motes) {
		return false
	}
	for i := range a.Motes {
		if a.Motes[i] != b.Motes[i] {
			return false
		}
	}
	return true
}

func TestRoundTrip(t *testing.T) {
	base := World{
		Tick: 7, Depth: -4096, Over: false,
		Cells: []uint8{1, 2, 3},
		Motes: []Mote{{ID: 1, HP: 10}},
	}
	cases := map[string]World{
		"nothing changed": base,
		"one scalar":      {Tick: 8, Depth: -4096, Cells: []uint8{1, 2, 3}, Motes: []Mote{{ID: 1, HP: 10}}},
		"a signed scale":  {Tick: 7, Depth: 9001, Cells: []uint8{1, 2, 3}, Motes: []Mote{{ID: 1, HP: 10}}},
		"a bool":          {Tick: 7, Depth: -4096, Over: true, Cells: []uint8{1, 2, 3}, Motes: []Mote{{ID: 1, HP: 10}}},
		"a byte slice":    {Tick: 7, Depth: -4096, Cells: []uint8{9}, Motes: []Mote{{ID: 1, HP: 10}}},
		"an emptied slice": {Tick: 7, Depth: -4096, Cells: nil, Motes: []Mote{{ID: 1, HP: 10}}},
		"a struct slice":  {Tick: 7, Depth: -4096, Cells: []uint8{1, 2, 3}, Motes: []Mote{{ID: 1, HP: 10}, {ID: 2, HP: -3}}},
		"everything":      {Tick: 99, Depth: 1, Over: true, Cells: []uint8{7, 7}, Motes: nil},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			d := DiffWorld(base, want)
			wire := d.AppendCBORTo(nil)

			var got WorldDelta
			if err := got.DecodeCBORFrom(wire); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Present != d.Present {
				t.Fatalf("present = %b, want %b", got.Present, d.Present)
			}

			// Patching the baseline with what the wire carried has to
			// reproduce the state the diff was taken against.
			out := base
			out.Cells = append([]uint8(nil), base.Cells...)
			out.Motes = append([]Mote(nil), base.Motes...)
			if err := ApplyWorldDelta(&out, got); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if !same(out, want) {
				t.Errorf("round trip gave %+v, want %+v", out, want)
			}
		})
	}
}

// A delta of nothing carries only the mask, which is what makes an idle
// tick nearly free.
func TestUnchangedCarriesOnlyTheMask(t *testing.T) {
	base := World{Tick: 1, Cells: []uint8{1}}
	d := DiffWorld(base, base)
	if d.Present != 0 {
		t.Fatalf("present = %b, want nothing", d.Present)
	}
	if n := len(d.AppendCBORTo(nil)); n > 3 {
		t.Errorf("an empty delta took %d bytes", n)
	}
}

// Trailing bytes mean the two ends disagree about the shape, and the
// array carries no field names to notice it any other way.
func TestTrailingDataIsRefused(t *testing.T) {
	d := DiffWorld(World{}, World{Tick: 3})
	wire := append(d.AppendCBORTo(nil), 0x00)
	var got WorldDelta
	if err := got.DecodeCBORFrom(wire); err == nil {
		t.Fatal("a trailing item was accepted")
	}
}
`)

	if out, err := runIn(dir, "go", "test", "./..."); err != nil {
		t.Fatalf("the generated delta does not round-trip:\n%s", out)
	}
}

// runIn runs a command in dir and returns what it said, so a failure
// reports the toolchain's own message rather than an exit code.
func runIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
