package game_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/importcheck"
)

// Importing Ebitengine from anything but the entry point breaks the
// build (rule:engine-import-confined-to-client-entry). Every game module
// carries this test, and each step of the tutorial is its own module, so
// each carries its own.
//
// The allowed entry is "." rather than a cmd/ directory: a step is one
// main package at its root with the rules underneath, so the window is
// the module itself. Everything below it — game/, msg/, distill/ — is
// held to the rule, which is what makes "the rules never see the engine"
// a fact about this directory rather than a habit.
//
// It lives beside the rules rather than at the root because a test in
// the root package would link the entry point, and linking the entry
// point links the engine: Ebitengine opens a window from an init
// function, which panics under a test binary with no display. The check
// reads the import graph with the go tool and needs nothing linked.
func TestModuleImportBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	cfg := importcheck.Default()
	cfg.Rules[0].AllowedEntries = append(cfg.Rules[0].AllowedEntries, ".")
	cfg.AllowedCgoEntries = append(cfg.AllowedCgoEntries, ".")
	importcheck.Enforce(t, "..", cfg)
}
