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

func spec(t *testing.T, reach, view string, seats int) *scaffold.Spec {
	t.Helper()
	sync := ""
	if modes := scaffold.SyncModesFor(reach); len(modes) > 0 {
		sync = scaffold.SyncDefaultFor(view)
		if !contains(modes, sync) {
			sync = modes[0]
		}
	}
	return &scaffold.Spec{
		Dir:           t.TempDir(),
		Module:        "example.com/mygame",
		Name:          "mygame",
		Reach:         reach,
		View:          view,
		Seats:         seats,
		SyncMode:      sync,
		FrameworkPath: frameworkRoot(t),
	}
}

// The acceptance criterion of requirement:project-scaffolding: what init
// writes compiles and its tests pass before anything is hand edited.
func TestGeneratedProjectBuildsAndPassesItsTests(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	// The four real quadrants of concept:view-arrangement, plus solo.
	for _, tc := range []struct {
		name  string
		reach string
		view  string
		seats int
	}{
		{"solo", "local", "shared", 1},
		{"couch_shared_camera", "local", "shared", 2},
		{"couch_split_camera", "local", "per_agent", 2},
		{"peer_shared_camera", "peer", "shared", 2},
		{"peer_star_four_players", "peer", "per_agent", 4},
		{"server_per_agent_camera", "server", "per_agent", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t, tc.reach, tc.view, tc.seats)
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
	s := spec(t, "local", "shared", 2)
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
	s := spec(t, "local", "shared", 2)
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
	s := spec(t, "local", "shared", 2)
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
		{"sync mode with no link to keep consistent", func(s *scaffold.Spec) {
			s.Reach, s.SyncMode = "local", "rollback"
		}, "no synchronization mode"},
		{"unknown reach", func(s *scaffold.Spec) { s.Reach = "mesh" }, "reach"},
		{"unknown camera", func(s *scaffold.Spec) { s.View = "isometric" }, "view arrangement"},
		{"rollback past a server hop", func(s *scaffold.Spec) {
			s.Reach, s.SyncMode = "server", "rollback"
		}, "sync mode"},
		{"a link with one seat", func(s *scaffold.Spec) {
			s.Reach, s.Seats, s.SyncMode = "peer", 1, "rollback"
		}, "at least 2 seats"},
		{"one seat with a per-seat camera", func(s *scaffold.Spec) {
			s.Seats, s.View = 1, "per_agent"
		}, "no camera to arrange"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t, "local", "shared", 2)
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

// The reach decides the entry point set and whether a synchronization
// mode exists; the camera decides neither. Sharing a camera across two
// machines does not remove the link — which is exactly the online
// fighting game case.
func TestReachDecidesStructureNotTheCamera(t *testing.T) {
	if got := scaffold.SyncModesFor("local"); len(got) != 0 {
		t.Errorf("local reach should have no synchronization mode, got %v", got)
	}
	if got := scaffold.SyncModesFor("peer"); len(got) == 0 {
		t.Error("a peer reach needs synchronization modes")
	}
	// Rollback belongs to the peer reach: re-simulating past a server hop
	// is not what it is for.
	if got := scaffold.SyncModesFor("server"); contains(got, "rollback") {
		t.Errorf("a server reach should not offer rollback, got %v", got)
	}
	// Any link wants a datagram channel, even for slow turns: cursors and
	// ping markers are presence, superseded rather than retransmitted.
	if !scaffold.NeedsUnreliable("peer") || !scaffold.NeedsUnreliable("server") {
		t.Error("a link should ask for a datagram channel")
	}
	if scaffold.NeedsUnreliable("local") {
		t.Error("local play has no link to carry datagrams")
	}
	for _, tgt := range scaffold.TargetsFor("local") {
		if tgt.Kind == "server" {
			t.Error("local play should not generate a server target")
		}
	}
	var server, client bool
	for _, tgt := range scaffold.TargetsFor("server") {
		server = server || tgt.Kind == "server"
		client = client || tgt.Kind == "client"
		if tgt.Kind == "server" && !tgt.Tagged() {
			t.Error("a server target should carry a second linkage form")
		}
	}
	if !server || !client {
		t.Error("a server reach should generate both a client and a server")
	}
	// A shared camera puts a mispredicted frame in front of everyone at
	// once, which is what rollback is for.
	if got := scaffold.SyncDefaultFor("shared"); got != "rollback" {
		t.Errorf("shared camera default = %q, want rollback", got)
	}
	if got := scaffold.SyncDefaultFor("per_agent"); got != "server_authoritative" {
		t.Errorf("per-agent camera default = %q", got)
	}
}

// The two linkage forms of one server come out of one directory, and the
// plain build must not reach the engine: that is the artifact that ships
// (rule:build-tag-only-for-linkage).
func TestServerBuildsBothLinkageForms(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	s := spec(t, "server", "per_agent", 4)
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
