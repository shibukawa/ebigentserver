package scaffold_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/scaffold"
)

// frameworkRoot is this checkout, so a generated project builds against
// the code under test rather than a published version.
func frameworkRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(wd)
}

func spec(t *testing.T, topology string, kinds ...string) *scaffold.Spec {
	t.Helper()
	targets := make([]scaffold.Target, 0, len(kinds))
	for _, k := range kinds {
		targets = append(targets, scaffold.Target{Name: k, Kind: k})
	}
	sync := scaffold.SyncModesFor(topology)[0]
	return &scaffold.Spec{
		Dir:           t.TempDir(),
		Module:        "example.com/mygame",
		Name:          "mygame",
		Topology:      topology,
		SyncMode:      sync,
		Targets:       targets,
		FrameworkPath: frameworkRoot(t),
	}
}

// The acceptance criterion of requirement:project-scaffolding: what init
// writes compiles and its tests pass before anything is hand edited.
func TestGeneratedProjectBuildsAndPassesItsTests(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	for _, tc := range []struct {
		topology string
		kinds    []string
	}{
		{"standalone", []string{"client", "simulation"}},
		{"listen", []string{"listen", "simulation"}},
		{"dedicated", []string{"dedicated", "simulation"}},
	} {
		t.Run(tc.topology, func(t *testing.T) {
			s := spec(t, tc.topology, tc.kinds...)
			generateAndTidy(t, s)
			goRun(t, s.Dir, "build", "./...")
			goRun(t, s.Dir, "test", "./...")
		})
	}
}

// A simulation entry has to run to completion without a human, a network,
// or an engine: it is what automated playtests go through.
func TestGeneratedSimulationRunsToCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	s := spec(t, "standalone", "simulation")
	generateAndTidy(t, s)
	out := goRun(t, s.Dir, "run", "./cmd/simulation")
	if !strings.Contains(out, "mygame simulation:") {
		t.Errorf("simulation output = %q", out)
	}
	// The placeholder game gives the first slot the odd cell, so the
	// outcome is decided rather than arbitrary. If this changes, the
	// rules changed.
	if !strings.Contains(out, "slot 1 win") {
		t.Errorf("simulation output = %q, want slot 1 to win", out)
	}
}

func TestGeneratedProjectWritesTheWholePipeline(t *testing.T) {
	s := spec(t, "standalone", "client", "simulation")
	res, err := scaffold.Generate(s)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// decision:ai-pipeline-always-scaffolded — never asked about, always
	// written, because a corpus cannot be collected retroactively.
	for _, want := range []string{
		"ebigent.toml", "go.mod", ".gitignore", "README.md",
		"game/game.go", "game/game_test.go",
		"cmd/client/main.go", "cmd/simulation/main.go",
		"behavior/chips.json", "corpus/.gitkeep",
		"skills/behavior-analyze/SKILL.md",
		"skills/behavior-analyze/scripts/validate_proposals.py",
	} {
		if !contains(res.Files, want) {
			t.Errorf("generated files are missing %q", want)
		}
	}
}

// A second init in a live project must not overwrite work.
func TestGenerateRefusesToOverwrite(t *testing.T) {
	s := spec(t, "standalone", "simulation")
	if _, err := scaffold.Generate(s); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	_, err := scaffold.Generate(s)
	if err == nil {
		t.Fatal("a second generate should refuse")
	}
	if !strings.Contains(err.Error(), "will not overwrite") {
		t.Errorf("err = %v", err)
	}
}

func TestSpecValidateRejectsUnreachableCombinations(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*scaffold.Spec)
		want string
	}{
		{"sync mode the topology cannot run", func(s *scaffold.Spec) {
			s.Topology = "standalone"
			s.SyncMode = "rollback"
		}, "does not support sync mode"},
		{"target the topology cannot use", func(s *scaffold.Spec) {
			s.Topology = "standalone"
			s.Targets = []scaffold.Target{{Name: "server", Kind: "dedicated"}}
		}, "cannot use a"},
		{"no targets", func(s *scaffold.Spec) { s.Targets = nil }, "at least one"},
		{"unknown topology", func(s *scaffold.Spec) { s.Topology = "mesh" }, "topology"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t, "standalone", "simulation")
			tc.mut(s)
			err := s.Validate()
			if err == nil {
				t.Fatalf("want an error naming %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to name %q", err, tc.want)
			}
		})
	}
}

// The wizard offers only combinations that exist, which is step 2 and 3
// of flow:project-init narrowing after step 1.
func TestWizardChoicesNarrowByTopology(t *testing.T) {
	if got := scaffold.SyncModesFor("standalone"); len(got) != 1 || got[0] != "server_authoritative" {
		t.Errorf("standalone sync modes = %v", got)
	}
	if contains(scaffold.TargetsFor("standalone"), "dedicated") {
		t.Error("standalone should not offer a dedicated target")
	}
	if !contains(scaffold.TargetsFor("dedicated"), "dedicated") {
		t.Error("the dedicated topology should offer a dedicated target")
	}
}

// generateAndTidy is steps 4 through 7 of flow:project-init. Module
// resolution runs from the local cache so the test needs no network.
func generateAndTidy(t *testing.T, s *scaffold.Spec) {
	t.Helper()
	if _, err := scaffold.Generate(s); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := scaffold.Tidy(s.Dir, []string{"GOFLAGS=-mod=mod", "GOPROXY=off"}); err != nil {
		t.Fatalf("tidy: %v", err)
	}
}

func goRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
