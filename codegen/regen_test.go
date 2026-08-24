package codegen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/ebigentserver/codegen"
)

// TestGeneratorReproducesTheCommittedFiles regenerates every wire package
// in this repository and requires the result to equal what is committed.
//
// It is the strongest check available on the generator. The delta and
// schema files under version control were produced by it, so any drift
// between the tree and what the code emits today surfaces here rather than
// as a wire mismatch on somebody's LAN — and it makes the committed output
// a fixture that never has to be maintained by hand.
func TestGeneratorReproducesTheCommittedFiles(t *testing.T) {
	asks, err := codegen.Stages("..")
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{
		"../tutorial/step2-lobby/msg",
		"../samples/dungeon/msg",
		"../samples/pong/msg",
		"../samples/rtslite/msg",
		"../samples/tron/msg",
		"../examples/phase0/msg",
	}
	for _, dir := range dirs {
		t.Run(dir, func(t *testing.T) {
			// A world reaches the delta generator two ways: named by a
			// rule set declaration, or asked for by hand where no
			// declaration names it. Both have to be covered or a
			// committed file would look unreachable.
			names, err := codegen.Discover(dir)
			if err != nil {
				t.Fatalf("discover: %v", err)
			}
			names = codegen.SortedNames(append(names, codegen.AskedWorlds(asks[dir])...))
			if len(names) == 0 {
				t.Fatalf("%s declares no world state", dir)
			}
			pkg, structs, err := codegen.Read(dir, names)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			src, err := codegen.Emit(pkg, structs)
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			same(t, filepath.Join(dir, "delta_gen.go"), src)
			same(t, filepath.Join(dir, "schema_gen.go"), codegen.EmitVersion(pkg, structs))

			if len(asks[dir]) == 0 {
				return
			}
			// The asks are generated too, so the committed file has to
			// be what the declaration produces today.
			name, err := codegen.PackageName(dir)
			if err != nil {
				t.Fatalf("package name: %v", err)
			}
			same(t, filepath.Join(dir, "wire_gen.go"), codegen.EmitAsks(name, asks[dir]))

			// And what the codec generator answered has to be whole. A
			// codec missing a member compiles and loses it on every
			// send, so the committed one is checked rather than trusted.
			if _, problems := codegen.CheckAsks(dir, asks[dir]); len(problems) > 0 {
				for _, p := range problems {
					t.Error(p)
				}
			}
		})
	}
}

// same compares one generated file with what regeneration produced, and
// reports the first line they differ on rather than the whole file.
func same(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s is missing: %v", path, err)
	}
	if string(got) == string(want) {
		return
	}
	gotLines, wantLines := lines(string(got)), lines(string(want))
	for i := range max(len(gotLines), len(wantLines)) {
		g, w := at(gotLines, i), at(wantLines, i)
		if g != w {
			t.Fatalf("%s line %d differs from what the generator emits now\n committed: %s\n generated: %s",
				path, i+1, g, w)
		}
	}
	t.Fatalf("%s differs in length: committed %d lines, generated %d", path, len(gotLines), len(wantLines))
}

func lines(s string) []string {
	var out []string
	for line := range splitLines(s) {
		out = append(out, line)
	}
	return out
}

func splitLines(s string) func(func(string) bool) {
	return func(yield func(string) bool) {
		start := 0
		for i := range len(s) {
			if s[i] == '\n' {
				if !yield(s[start:i]) {
					return
				}
				start = i + 1
			}
		}
		if start < len(s) {
			yield(s[start:])
		}
	}
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<missing>"
}
