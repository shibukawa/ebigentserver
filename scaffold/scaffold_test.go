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

func spec(t *testing.T, style string, seats int, localMulti bool) *scaffold.Spec {
	t.Helper()
	seats = scaffold.SeatsForStyle(style, seats)
	sync := ""
	if modes := scaffold.SyncModesFor(seats); len(modes) > 0 {
		sync = scaffold.SyncDefaultFor(style)
		if !contains(modes, sync) {
			sync = modes[0]
		}
	}
	return &scaffold.Spec{
		Dir:              t.TempDir(),
		Module:           "example.com/mygame",
		Name:             "mygame",
		Style:            style,
		Seats:            seats,
		LocalMultiplayer: localMulti,
		SyncMode:         sync,
		FrameworkPath:    frameworkRoot(t),
	}
}

// The acceptance criterion of requirement:project-scaffolding: what init
// writes compiles and its tests pass before anything is hand edited.
func TestGeneratedProjectBuildsAndPassesItsTests(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	// The four real quadrants of concept:view-arrangement, plus solo.
	// The five code patterns: solo, then duo and multi, each with or
	// without several players on one machine.
	for _, tc := range []struct {
		name       string
		style      string
		seats      int
		localMulti bool
	}{
		{"solo", "solo", 0, false},
		{"duo_one_machine", "duo", 0, true},
		{"duo_own_machines", "duo", 0, false},
		{"multi_one_machine", "multi", 4, true},
		{"multi_own_machines", "multi", 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t, tc.style, tc.seats, tc.localMulti)
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
	s := spec(t, "duo", 0, true)
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
	s := spec(t, "solo", 0, false)
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
	s := spec(t, "duo", 0, true)
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
			s.Style, s.Seats, s.LocalMultiplayer, s.SyncMode = "solo", 1, false, "rollback"
		}, "no synchronization mode"},
		{"unknown style", func(s *scaffold.Spec) { s.Style = "co_op" }, "play style"},
		{"a style whose seat count was overridden", func(s *scaffold.Spec) {
			s.Style, s.Seats = "duo", 3
		}, "declares 2 seats"},
		{"one player sharing a machine", func(s *scaffold.Spec) {
			s.Style, s.Seats, s.LocalMultiplayer, s.SyncMode = "solo", 1, true, ""
		}, "cannot share a machine"},
		{"rollback past two seats", func(s *scaffold.Spec) {
			s.Style, s.Seats, s.SyncMode = "multi", 4, "rollback"
		}, "sync mode"},
		{"no seats", func(s *scaffold.Spec) { s.Style, s.Seats = "multi", 0 }, "at least one seat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t, "duo", 0, true)
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

// The style decides the entry point set and which synchronization modes
// are reachable; whether several players share a machine decides neither.
// A game can seat two people at one keyboard and still play online, which
// is why the two are separate questions.
func TestStyleDecidesStructureNotTheSeating(t *testing.T) {
	if got := scaffold.SyncModesFor(1); len(got) != 0 {
		t.Errorf("one seat should have no synchronization mode, got %v", got)
	}
	// Two seats is the one case where a peer link is genuinely one hop,
	// which is what rollback and delay buffering assume.
	if got := scaffold.SyncModesFor(2); !contains(got, "rollback") {
		t.Errorf("two seats should offer rollback, got %v", got)
	}
	// Past two, every exchange is two hops whichever host is chosen, so
	// the one-hop netcodes stop being reachable.
	if got := scaffold.SyncModesFor(4); contains(got, "rollback") {
		t.Errorf("four seats should not offer rollback, got %v", got)
	}
	// Any link wants a datagram channel, even for slow turns: cursors and
	// ping markers are presence, superseded rather than retransmitted.
	if !scaffold.NeedsUnreliable(2) || scaffold.NeedsUnreliable(1) {
		t.Error("the datagram requirement should follow whether a link exists")
	}
	// duo fixes its own seat count; multi takes one.
	if got := scaffold.SeatsForStyle("duo", 7); got != 2 {
		t.Errorf("duo seats = %d, want 2", got)
	}
	if got := scaffold.SeatsForStyle("multi", 6); got != 6 {
		t.Errorf("multi seats = %d, want the given 6", got)
	}
	// Two seats default to a playing host; past two, to one that can be
	// trusted with the result. Both remain run values either way.
	if got := scaffold.TopologyForSeats(2); got != "listen" {
		t.Errorf("two-seat topology = %q, want listen", got)
	}
	if got := scaffold.TopologyForSeats(4); got != "dedicated" {
		t.Errorf("four-seat topology = %q, want dedicated", got)
	}
	for _, tgt := range scaffold.TargetsFor(1) {
		if tgt.Kind == "server" {
			t.Error("solo play should not generate a server target")
		}
	}
	var server, client bool
	for _, tgt := range scaffold.TargetsFor(4) {
		server = server || tgt.Kind == "server"
		client = client || tgt.Kind == "client"
		if tgt.Kind == "server" && !tgt.Tagged() {
			t.Error("a server target should carry a second linkage form")
		}
	}
	if !server || !client {
		t.Error("a linked project should generate both a client and a server")
	}
	// Two players reach each other in one hop, which is what rollback
	// assumes; past two it is two hops whichever host is chosen.
	if got := scaffold.SyncDefaultFor("duo"); got != "rollback" {
		t.Errorf("duo default = %q, want rollback", got)
	}
	if got := scaffold.SyncDefaultFor("multi"); got != "server_authoritative" {
		t.Errorf("multi default = %q", got)
	}
}

// The two linkage forms of one server come out of one directory, and the
// plain build must not reach the engine: that is the artifact that ships
// (rule:build-tag-only-for-linkage).
func TestServerBuildsBothLinkageForms(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	s := spec(t, "multi", 4, false)
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
