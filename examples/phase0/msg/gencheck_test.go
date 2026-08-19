package msg_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/generator"
)

// Phase 0 completion criterion: adding a float field to a simulation struct
// breaks the build (rule:codegen-rejects-nondeterministic-types). The gate is
// the generation pass, so the test reproduces this package in a temp module,
// adds the float, and requires the generator to refuse.
func TestGeneratorRejectsFloatField(t *testing.T) {
	requireGenerationError(t, "Speed float64", "float")
}

// A bare int is 64-bit on the host and 32-bit on wasm, so the two ends of a
// connection would disagree about what fits.
func TestGeneratorRejectsBareInt(t *testing.T) {
	requireGenerationError(t, "Count int", "int")
}

// Go randomizes map iteration order, so traversal and diff output vary per run.
func TestGeneratorRejectsMapField(t *testing.T) {
	requireGenerationError(t, "Scores map[uint32]int32", "map")
}

// requireGenerationError copies this package's types.go into a temp module,
// injects one extra field into PlayerInput, and asserts generation fails
// with a message naming the problem.
func requireGenerationError(t *testing.T, field, wantInError string) {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}

	src, err := os.ReadFile("types.go")
	if err != nil {
		t.Fatal(err)
	}
	const anchor = "Buttons uint16"
	if !strings.Contains(string(src), anchor) {
		t.Fatalf("types.go no longer contains anchor %q; update the test", anchor)
	}
	mutated := strings.Replace(string(src), anchor, anchor+"\n\t"+field, 1)

	dir := t.TempDir()
	writeFixtureModule(t, dir, mutated)

	_, err = generator.AnalyzeCborCodecsWithOptions(dir, generator.DefaultOptions())
	if err == nil {
		t.Fatalf("generator accepted a struct with %q; determinism gate is gone", field)
	}
	if !strings.Contains(err.Error(), wantInError) {
		t.Errorf("error should name the offending kind %q, got: %v", wantInError, err)
	}
	t.Logf("rejected as expected: %v", err)
}

func writeFixtureModule(t *testing.T, dir, source string) {
	t.Helper()
	gomod := `module example.com/floatcheck

go 1.26

require (
	github.com/shibukawa/tinybind-go ` + moduleVersion(t, "github.com/shibukawa/tinybind-go") + `
	github.com/shibukawa/tinygodriver ` + moduleVersion(t, "github.com/shibukawa/tinygodriver") + `
)
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	runGo(t, dir, "mod", "tidy")
}
