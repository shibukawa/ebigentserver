package codegen_test

import (
	"slices"
	"testing"

	"github.com/shibukawa/ebigentserver/codegen"
)

// TestDiscoverFindsTheWorldStates covers the declaration this generator
// reads: a type asked for the map shape is a world state, and one asked
// for the array shape is a message with no baseline to diff against.
//
// It runs over the real packages, so a game that already ships cannot
// have a world state the generator would silently skip.
func TestDiscoverFindsTheWorldStates(t *testing.T) {
	cases := map[string]struct {
		want, absent []string
	}{
		"../tutorial/step2-lobby/msg": {want: []string{"TTTWorld"}, absent: []string{"Move"}},
		"../samples/pong/msg":         {want: []string{"PongState"}, absent: []string{"PaddleInput"}},
		"../samples/dungeon/msg": {
			want:   []string{"AdventurerView", "DMView", "DungeonState"},
			absent: []string{"ActionInput"},
		},
		"../samples/rtslite/msg": {want: []string{"PlayerView", "RTSState"}, absent: []string{"Command"}},
		"../samples/tron/msg":    {want: []string{"TronState"}, absent: []string{"TurnInput"}},
		"../examples/phase0/msg": {want: []string{"WorldState"}, absent: []string{"PlayerInput"}},
	}
	for dir, tc := range cases {
		t.Run(dir, func(t *testing.T) {
			got, err := codegen.Discover(dir)
			if err != nil {
				t.Fatalf("discover: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("discovered %v, want %v", got, tc.want)
			}
			for _, no := range tc.absent {
				if slices.Contains(got, no) {
					t.Errorf("%s travels as an array and needs no delta", no)
				}
			}
		})
	}
}

// TestDiscoverIgnoresItsOwnOutput keeps a second run from reading back the
// types it emitted and treating them as new declarations.
func TestDiscoverIgnoresItsOwnOutput(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "types.go", `package probe

import "github.com/shibukawa/tinybind-go/cborbind"

type World struct{ Tick uint64 }

func DecodeWorld(data []byte) (World, error) { return cborbind.DecodeCBORInMapFrom[World](data) }
`)
	write(t, dir, "delta_gen.go", `package probe

import "github.com/shibukawa/tinybind-go/cborbind"

type Ghost struct{ Tick uint64 }

func DecodeGhost(data []byte) (Ghost, error) { return cborbind.DecodeCBORInMapFrom[Ghost](data) }
`)
	got, err := codegen.Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"World"}) {
		t.Errorf("discovered %v, want only the hand-written declaration", got)
	}
}

// TestDiscoverReadsAnInferredCall covers the append side, whose type
// argument is inferred from the value rather than written out.
func TestDiscoverReadsAnInferredCall(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "types.go", `package probe

import "github.com/shibukawa/tinybind-go/cborbind"

type Board struct{ Tick uint64 }

func AppendBoard(dst []byte, v Board) []byte { return cborbind.AppendCBORInMapTo(dst, v) }
`)
	got, err := codegen.Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"Board"}) {
		t.Errorf("discovered %v, want Board", got)
	}
}
