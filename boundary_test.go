package ebigentserver_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/importcheck"
)

// Phase 0 completion criterion: importing Ebitengine from anything but a
// client entry point breaks the build. This test holds the framework module
// itself to rule:engine-import-confined-to-client-entry; every game module
// carries the same test with its own entry patterns.
func TestModuleImportBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	cfg := importcheck.Default()
	exampleEntries := []string{
		"examples/*/cmd/*client*",
		"examples/*/cmd/*listen*",
		"examples/*/cmd/*static*",
	}
	cfg.Rules[0].AllowedEntries = append(cfg.Rules[0].AllowedEntries, exampleEntries...)
	cfg.AllowedCgoEntries = append(cfg.AllowedCgoEntries, exampleEntries...)
	importcheck.Enforce(t, ".", cfg)
}
