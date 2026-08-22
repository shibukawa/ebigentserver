package codegen_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/codegen"
)

// TestReadResolvesEveryFieldShapeTheSamplesUse walks the real wire types
// rather than a fixture, so a field shape a game already ships cannot be
// one the delta generator silently cannot carry.
func TestReadResolvesEveryFieldShapeTheSamplesUse(t *testing.T) {
	cases := []struct {
		dir   string
		types []string
		want  map[string]codegen.Kind
	}{
		{
			"../tutorial/step2-lobby/msg", []string{"TTTState"},
			map[string]codegen.Kind{
				"Cells": codegen.KindBytes, "Turn": codegen.KindUint, "Over": codegen.KindBool,
			},
		},
		{
			// Named scalars over int64 — the fixed-point scales of
			// decision:fixed-point-numeric-representation — travel as
			// integers and must not be mistaken for something else.
			"../samples/pong/msg", []string{"PongState"},
			map[string]codegen.Kind{"BallX": codegen.KindInt, "Tick": codegen.KindUint},
		},
		{
			// A slice of structs is carried whole, so the generator has
			// to know the element type to encode it.
			"../samples/dungeon/msg", []string{"DungeonState"},
			map[string]codegen.Kind{"Traps": codegen.KindStructSlice, "Walls": codegen.KindBytes},
		},
		{"../samples/tron/msg", []string{"TronState"}, nil},
		{"../samples/rtslite/msg", []string{"RTSState", "PlayerView"}, nil},
		{"../examples/phase0/msg", []string{"WorldState"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			_, out, err := codegen.Read(tc.dir, tc.types)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if len(out) != len(tc.types) {
				t.Fatalf("read %d structs, want %d", len(out), len(tc.types))
			}
			got := map[string]codegen.Kind{}
			for _, f := range out[0].Fields {
				got[f.Name] = f.Kind
			}
			for name, want := range tc.want {
				if got[name] != want {
					t.Errorf("%s is kind %d, want %d", name, got[name], want)
				}
			}
			for _, f := range out[0].Fields {
				if f.Kind == codegen.KindStructSlice && f.Elem == "" {
					t.Errorf("%s is a struct slice with no element type", f.Name)
				}
			}
		})
	}
}

// TestReadRefusesWhatADeltaCannotCarry covers the gate that used to be
// the encoder's. The removed CBOR profile refused floats outright; with
// no profile left, generation is the only place rule:no-float-in-simulation
// can still be enforced.
func TestReadRefusesWhatADeltaCannotCarry(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module probe\n\ngo 1.24\n")
	write(t, dir, "types.go", `package probe

type Drifting struct {
	Tick  uint64
	Speed float64
}
`)
	_, _, err := codegen.Read(dir, []string{"Drifting"})
	if err == nil {
		t.Fatal("a float field was accepted into a delta")
	}
	var unsupported *codegen.ErrUnsupported
	if !as(err, &unsupported) {
		t.Fatalf("err = %v, want it to name the field", err)
	}
	if unsupported.Field != "Speed" {
		t.Errorf("refused %q, want Speed", unsupported.Field)
	}
}

// TestReadRefusesATypeItCannotFind keeps a misspelled name from producing
// an empty file that compiles and carries nothing.
func TestReadRefusesATypeItCannotFind(t *testing.T) {
	if _, _, err := codegen.Read("../tutorial/step2-lobby/msg", []string{"TTTStat"}); err == nil {
		t.Fatal("a type that does not exist was accepted")
	}
}
