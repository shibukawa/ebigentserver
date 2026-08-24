package ebigentserver_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/importcheck"
)

// Phase 0 completion criterion: importing Ebitengine from anything but a
// client entry point breaks the build. This test holds the framework module
// itself to rule:engine-import-confined-to-client-entry; every game module
// carries the same test with its own entry patterns.
//
// The tutorial is not covered here. Each step is its own module — that is
// what lets step 2 run `ebigent init` for real — so each carries the test
// in its own game package, allowing "." as the entry.
func TestModuleImportBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	cfg := importcheck.Default()
	exampleEntries := []string{
		"examples/*/cmd/*client*",
		"examples/*/cmd/*listen*",
		"examples/*/cmd/*static*",
		"samples/*/cmd/*client*",
		"samples/*/cmd/*listen*",
		"samples/*/cmd/*static*",
		// run/eb is the engine half of api:run-wrapper: the framework's
		// own adapter, and the reason a game's entry point no longer has
		// to write one. It is where the boundary is drawn rather than a
		// hole in it — package run, which every headless build links, is
		// checked by this same pass and must stay clean.
		"run/eb",
	}
	cfg.Rules[0].AllowedEntries = append(cfg.Rules[0].AllowedEntries, exampleEntries...)
	cfg.AllowedCgoEntries = append(cfg.AllowedCgoEntries, exampleEntries...)
	importcheck.Enforce(t, ".", cfg)
}
