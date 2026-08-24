package codegen_test

import (
	"slices"
	"testing"

	"github.com/shibukawa/ebigentserver/codegen"
)

// TestDiscoverFindsTheWorldStates covers what is still asked for by
// hand: a type asked for the map shape is a world state, and one asked
// for the array shape is a message with no baseline to diff against.
//
// What remains here after requirement:stage-declares-its-wire are the
// per-role projections a game synchronises alongside its world. No rule
// set declaration names them, so the ask is written and this is what
// reads it.
func TestDiscoverFindsTheWorldStates(t *testing.T) {
	cases := map[string]struct {
		want, absent []string
	}{
		"../tutorial/step2-lobby/msg": {want: nil, absent: []string{"Move", "TTTWorld"}},
		"../samples/pong/msg":         {want: nil, absent: []string{"PaddleInput", "PongState"}},
		"../samples/dungeon/msg": {
			want:   []string{"AdventurerView", "DMView"},
			absent: []string{"ActionInput", "DungeonState"},
		},
		"../samples/rtslite/msg": {want: []string{"PlayerView"}, absent: []string{"Command", "RTSState"}},
		"../samples/tron/msg":    {want: nil, absent: []string{"TurnInput", "TronState"}},
		"../examples/phase0/msg": {want: []string{"WorldState"}, absent: []string{"PlayerInput"}},
	}
	for dir, tc := range cases {
		t.Run(dir, func(t *testing.T) {
			got, err := codegen.Discover(dir)
			if err != nil {
				t.Fatalf("discover: %v", err)
			}
			if len(got) != len(tc.want) || (len(got) > 0 && !slices.Equal(got, tc.want)) {
				t.Fatalf("discovered %v, want %v", got, tc.want)
			}
			for _, no := range tc.absent {
				if slices.Contains(got, no) {
					t.Errorf("%s is not asked for by hand here", no)
				}
			}
		})
	}
}

// TestStagesReadTheRuleSetDeclarations is the other half: what a stage
// asks for comes from the assertion its rules already carry, and nothing
// beside the types has to say it (requirement:stage-declares-its-wire).
func TestStagesReadTheRuleSetDeclarations(t *testing.T) {
	asks, err := codegen.Stages("..")
	if err != nil {
		t.Fatal(err)
	}
	// The tutorial writes its rule set in one package and its types in
	// another, so this also covers following the declaration across the
	// import to the package that owns the type.
	for dir, want := range map[string][]codegen.Ask{
		"../tutorial/step2-lobby/msg": {
			{Type: "Move", Shape: codegen.Array, Position: "action"},
			{Type: "TTTWorld", Shape: codegen.Map, Position: "world"},
		},
		// dungeon names its types through a local alias, which has to
		// resolve to the same package the wire types live in.
		"../samples/dungeon/msg": {
			{Type: "ActionInput", Shape: codegen.Array, Position: "action"},
			{Type: "DungeonState", Shape: codegen.Map, Position: "world"},
		},
	} {
		if got := asks[dir]; !slices.Equal(got, want) {
			t.Errorf("%s asks for %v, want %v", dir, got, want)
		}
	}
	// The interface's own definition and a struct field holding one name
	// the same three parameters without being a stage.
	for _, no := range []string{"../session", "../run"} {
		if got := asks[no]; len(got) > 0 {
			t.Errorf("%s is not a stage but asks for %v", no, got)
		}
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
