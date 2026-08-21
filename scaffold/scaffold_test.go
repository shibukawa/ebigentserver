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

func spec(t *testing.T, shape string) *scaffold.Spec {
	t.Helper()
	return &scaffold.Spec{
		Dir:           t.TempDir(),
		Module:        "example.com/mygame",
		Name:          "mygame",
		Shape:         shape,
		SyncMode:      scaffold.SyncModesFor(shape)[0],
		FrameworkPath: frameworkRoot(t),
	}
}

// The acceptance criterion of requirement:project-scaffolding: what init
// writes compiles and its tests pass before anything is hand edited.
func TestGeneratedProjectBuildsAndPassesItsTests(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	for _, shape := range scaffold.Shapes {
		t.Run(shape, func(t *testing.T) {
			s := spec(t, shape)
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
	s := spec(t, "solo")
	generateAndTidy(t, s)
	out := goRun(t, s.Dir, "run", "./cmd/simulation")
	if !strings.Contains(out, "mygame simulation:") {
		t.Errorf("simulation output = %q", out)
	}
	// Two identical bots flying the same seeded pipe field tie, and both
	// reach the target. Anything else means either the physics stopped
	// being deterministic or the placeholder bot stopped being able to
	// fly — both worth failing over.
	if !strings.Contains(out, "slot 1 draw (10 pipes)") || !strings.Contains(out, "slot 2 draw (10 pipes)") {
		t.Errorf("simulation output = %q, want both bots to clear 10 pipes and tie", out)
	}
}

func TestGeneratedProjectWritesTheWholePipeline(t *testing.T) {
	s := spec(t, "solo")
	res, err := scaffold.Generate(s)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// decision:ai-pipeline-always-scaffolded — never asked about, always
	// written, because a corpus cannot be collected retroactively.
	for _, want := range []string{
		"ebigent.toml", "go.mod", ".gitignore", "README.md",
		"game/game.go", "game/game_test.go", "boundary_test.go",
		"cmd/game/main.go", "cmd/simulation/main.go",
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
	s := spec(t, "solo")
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
		{"sync mode the shape cannot run", func(s *scaffold.Spec) {
			s.Shape = "solo"
			s.SyncMode = "rollback"
		}, "does not support sync mode"},
		{"unknown shape", func(s *scaffold.Spec) { s.Shape = "mesh" }, "shape"},
		{"unknown sync mode", func(s *scaffold.Spec) { s.SyncMode = "lockstep" }, "sync mode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t, "solo")
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

// The shape decides the entry point set, and step 2 narrows to the
// synchronization modes that shape can actually run.
func TestShapeDecidesTargetsAndNarrowsSyncModes(t *testing.T) {
	if got := scaffold.SyncModesFor("solo"); len(got) != 1 || got[0] != "server_authoritative" {
		t.Errorf("solo sync modes = %v", got)
	}
	// solo is the only shape with no host to choose. Two seats do not
	// imply peer to peer: a duo can be server hosted exactly as a multi
	// can, which is why both carry the same tagged server entry
	// (concept:deployment-combination).
	for _, tgt := range scaffold.TargetsFor("solo") {
		if tgt.Kind == "server" {
			t.Error("solo should not generate a server target")
		}
	}
	for _, shape := range []string{"duo", "multi"} {
		var server, client bool
		for _, tgt := range scaffold.TargetsFor(shape) {
			server = server || tgt.Kind == "server"
			client = client || tgt.Kind == "client"
			if tgt.Kind == "server" && !tgt.Tagged() {
				t.Errorf("%s: a server target should carry a second linkage form", shape)
			}
		}
		if !server || !client {
			t.Errorf("%s should generate both a client and a server", shape)
		}
	}
}

// The two linkage forms of one server come out of one directory, and the
// plain build must not reach the engine: that is the artifact that ships
// (rule:build-tag-only-for-linkage).
func TestServerBuildsBothLinkageForms(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	s := spec(t, "multi")
	generateAndTidy(t, s)
	for _, f := range []string{"cmd/server/main.go", "cmd/server/listen.go", "cmd/server/headless.go"} {
		if _, err := os.Stat(filepath.Join(s.Dir, f)); err != nil {
			t.Fatalf("missing %s", f)
		}
	}
	goRun(t, s.Dir, "build", "-o", filepath.Join(s.Dir, "bin", "headless"), "./cmd/server")
	goRun(t, s.Dir, "build", "-tags", "listen", "-o", filepath.Join(s.Dir, "bin", "listen"), "./cmd/server")

	// The plain build is the one the import graph check inspects, and it
	// must not reach the engine at all.
	deps := goRun(t, s.Dir, "list", "-deps", "./cmd/server")
	if strings.Contains(deps, "hajimehoshi/ebiten") {
		t.Error("the headless server links the engine; the listen tag should be the only thing that does")
	}
	tagged := goRun(t, s.Dir, "list", "-deps", "-tags", "listen", "./cmd/server")
	if !strings.Contains(tagged, "hajimehoshi/ebiten") {
		t.Error("the listen build should link the engine")
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
