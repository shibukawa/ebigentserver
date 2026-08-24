package scaffold_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/config/buildconf"
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

func spec(t *testing.T, style string, seats int, sharedScreen bool) *scaffold.Spec {
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
		Dir:           t.TempDir(),
		Agent:         "claude",
		Module:        "example.com/mygame",
		Name:          "mygame",
		Style:         style,
		Seats:         seats,
		SharedScreen:  sharedScreen,
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
	// The five code patterns: solo, then duo and multi, each with or
	// without several players on one machine.
	for _, tc := range []struct {
		name   string
		style  string
		seats  int
		shared bool
	}{
		{"solo", "solo", 0, false},
		{"duo_one_stage", "duo", 0, true},
		{"duo_own_views", "duo", 0, false},
		{"multi_one_stage", "multi", 4, true},
		{"multi_own_views", "multi", 4, false},
		{"multi_at_two_seats", "multi", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t, tc.style, tc.seats, tc.shared)
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
		"ebigent.toml", ".gitignore", "README.md",
		"game/game.go", "game/game_test.go", "boundary_test.go",
		"cmd/mygame/main.go", "cmd/mygame/play.go", "cmd/simulation/main.go",
		"behavior/chips.json", "corpus/.gitkeep",
		".claude/skills/behavior-analyze/SKILL.md",
		".claude/skills/behavior-analyze/scripts/validate_proposals.py",
	} {
		if !contains(res.Files, want) {
			t.Errorf("generated files are missing %q", want)
		}
	}
}

// The skill lands where the developer's environment looks for it, which
// is why the path is not a setting: nothing has to be told about it.
func TestSkillLandsWhereTheEnvironmentLooks(t *testing.T) {
	for agent, want := range map[string]string{
		"claude": ".claude/skills/behavior-analyze/SKILL.md",
		"other":  ".agents/skills/behavior-analyze/SKILL.md",
	} {
		s := spec(t, "solo", 0, false)
		s.Agent = agent
		res, err := scaffold.Generate(s)
		if err != nil {
			t.Fatalf("%s: %v", agent, err)
		}
		if !contains(res.Files, want) {
			t.Errorf("%s: skill not at %s; got %v", agent, want, res.Files)
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
			s.Style, s.Seats, s.SharedScreen, s.SyncMode = "solo", 1, false, "rollback"
		}, "no synchronization mode"},
		{"unknown style", func(s *scaffold.Spec) { s.Style = "co_op" }, "play style"},
		{"unknown agent environment", func(s *scaffold.Spec) { s.Agent = "emacs" }, "agent environment"},
		{"a style whose seat count was overridden", func(s *scaffold.Spec) {
			s.Style, s.Seats = "duo", 3
		}, "declares 2 seats"},
		{"one player sharing a screen", func(s *scaffold.Spec) {
			s.Style, s.Seats, s.SharedScreen, s.SyncMode = "solo", 1, true, ""
		}, "nobody to share a screen with"},
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
	// duo starts at a playing host, because one hop is why it is a style.
	// multi starts at a dedicated one whatever its seat count — including
	// two, where picking multi says latency is not what decides the game.
	// Both are starting values: a player can hold a session of any size,
	// which is what concept:static-host-mode is.
	if got := scaffold.TopologyForStyle("duo"); got != "listen" {
		t.Errorf("duo topology = %q, want listen", got)
	}
	if got := scaffold.TopologyForStyle("multi"); got != "dedicated" {
		t.Errorf("multi topology = %q, want dedicated", got)
	}

	// Two players who are not latency-bound are multi with two seats, not
	// duo. Both offer the one-hop netcodes, since the seat count is what
	// makes them reachable, but only duo defaults to one.
	if got := scaffold.SyncModesFor(2); !contains(got, "rollback") {
		t.Errorf("two seats should offer rollback whichever style asked, got %v", got)
	}
	if got := scaffold.SyncDefaultFor("multi"); got == "rollback" {
		t.Error("multi should not default to rollback even at two seats")
	}
	// Solo has nothing to host, so its entry is a plain client with no
	// second linkage form.
	for _, tgt := range scaffold.TargetsFor(1) {
		if tgt.Tagged() {
			t.Errorf("solo play should have no headless form, got %+v", tgt)
		}
		if tgt.Kind == "listen" {
			t.Error("solo play hosts nothing")
		}
	}
	// A linked project has one playable entry that also hosts, and the
	// same directory builds headless under the tag.
	var playable scaffold.Target
	for _, tgt := range scaffold.TargetsFor(4) {
		if tgt.Kind == "listen" {
			playable = tgt
		}
	}
	if playable.Name == "" {
		t.Fatal("a linked project needs an entry that plays and hosts")
	}
	if !playable.Tagged() {
		t.Error("the playable entry should also build headless under the tag")
	}
	if playable.DedicatedName() == playable.Name {
		t.Error("the two forms need distinct target names in ebigent.toml")
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
	for _, f := range []string{"cmd/mygame/main.go", "cmd/mygame/play.go", "cmd/mygame/dedicated.go"} {
		if _, err := os.Stat(filepath.Join(s.Dir, f)); err != nil {
			t.Fatalf("missing %s", f)
		}
	}
	goRun(t, s.Dir, "build", "-o", filepath.Join(s.Dir, "bin", "play"), "./cmd/mygame")
	goRun(t, s.Dir, "build", "-tags", "dedicated", "-o", filepath.Join(s.Dir, "bin", "server"), "./cmd/mygame")

	// The tag takes the renderer away rather than adding one: the plain
	// build is what a developer runs, and the tagged one is the artifact
	// that ships headless.
	play := goRun(t, s.Dir, "list", "-deps", "./cmd/mygame")
	if !strings.Contains(play, "hajimehoshi/ebiten") {
		t.Error("the plain build should render")
	}
	headless := goRun(t, s.Dir, "list", "-deps", "-tags", "dedicated", "./cmd/mygame")
	if strings.Contains(headless, "hajimehoshi/ebiten") {
		t.Error("the dedicated build links the engine; the tag exists to drop it")
	}
}

// generateAndTidy is steps 4 through 7 of flow:project-init. Module
// resolution runs from the local cache so the test needs no network.
func generateAndTidy(t *testing.T, s *scaffold.Spec) {
	t.Helper()
	if _, err := scaffold.Generate(s); err != nil {
		t.Fatalf("generate: %v", err)
	}
	// A local framework checkout, so resolution reads that directory's
	// requirements and needs no network.
	if err := scaffold.InitModule(s.Dir, s, []string{"GOFLAGS=-mod=mod", "GOPROXY=off"}); err != nil {
		t.Fatalf("module init: %v", err)
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

// The generated go.mod is built by the go tool, not written from a
// template, so its go directive follows the installed toolchain rather
// than whatever a template froze.
func TestGeneratedModuleIsBuiltByTheGoTool(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	s := spec(t, "solo", 0, false)
	generateAndTidy(t, s)

	body, err := os.ReadFile(filepath.Join(s.Dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "module example.com/mygame") {
		t.Errorf("go.mod names the wrong module:\n%s", got)
	}
	// go mod init writes a directive for the toolchain in hand; a
	// template could only ever name the one it was written against.
	installed := strings.TrimPrefix(runtime.Version(), "go")
	major := strings.Join(strings.SplitN(installed, ".", 3)[:2], ".")
	if !strings.Contains(got, "go "+major) {
		t.Errorf("go directive does not follow the installed toolchain %s:\n%s", installed, got)
	}
	// Everything the sources import has to be required, whether the
	// framework pulled it in or the game did.
	for _, want := range []string{
		"github.com/shibukawa/ebigentserver",
		"github.com/shibukawa/fixmath",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("go.mod is missing %s:\n%s", want, got)
		}
	}
}

// A project with a rendering entry needs the engine resolvable; one
// without it should not be asked to fetch it.
func TestDirectDepsFollowTheTargets(t *testing.T) {
	withClient := spec(t, "solo", 0, false).DirectDeps()
	if !contains(withClient, "github.com/hajimehoshi/ebiten/v2") {
		t.Errorf("a rendering project needs the engine, got %v", withClient)
	}
	if !contains(withClient, "github.com/shibukawa/fixmath") {
		t.Errorf("every project uses fixed point, got %v", withClient)
	}
}

// ModulePath answers the question init asks before it does anything
// else, so the three answers it can give are all worth pinning: no
// module, a module, and a file that claims to declare one and does not.
func TestModulePathReadsWhatGoModDeclares(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		ok   bool
		fail bool
	}{
		{name: "absent"},
		{name: "plain", body: "module example.com/pong\n\ngo 1.25\n", want: "example.com/pong", ok: true},
		{name: "quoted", body: "module \"example.com/pong\"\n", want: "example.com/pong", ok: true},
		{name: "after a comment", body: "// a game\nmodule example.com/pong\n", want: "example.com/pong", ok: true},
		{name: "declares nothing", body: "go 1.25\n", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.body != "" || tc.fail {
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, ok, err := scaffold.ModulePath(dir)
			switch {
			case tc.fail && err == nil:
				t.Fatalf("a go.mod with no module path should be an error, got %q", got)
			case tc.fail:
				return
			case err != nil:
				t.Fatal(err)
			}
			if got != tc.want || ok != tc.ok {
				t.Errorf("ModulePath = %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// An adopted module gets the framework's own files and keeps its own
// sources, which is the whole of the difference between init's two jobs.
func TestAdoptedProjectKeepsItsOwnSources(t *testing.T) {
	s := spec(t, "solo", 0, false)
	s.Adopt = true
	s.Detected = []scaffold.Target{{Name: "pong", Dir: "cmd/pong", Kind: "client", Path: "./cmd/pong"}}
	res, err := scaffold.Generate(s)
	if err != nil {
		t.Fatal(err)
	}
	written := map[string]bool{}
	for _, f := range res.Files {
		written[f] = true
	}
	for _, want := range []string{"ebigent.toml", "behavior/chips.json", "corpus/.gitkeep", "cmd/distill/main.go"} {
		if !written[want] {
			t.Errorf("adopting a module did not write %s; wrote %v", want, res.Files)
		}
	}
	// The placeholder game exists to be replaced, and a module that has
	// its own game has already done that. README.md and .gitignore
	// belong to whoever started the repository.
	for _, unwanted := range []string{"game/game.go", "game/game_test.go", "boundary_test.go", "README.md", ".gitignore", "cmd/pong/main.go", "cmd/simulation/main.go"} {
		if written[unwanted] {
			t.Errorf("adopting a module wrote %s, which is not init's to write", unwanted)
		}
	}
	// The detected entry is what the configuration declares, because a
	// generated cmd/<name> is not there to declare.
	body, err := os.ReadFile(filepath.Join(s.Dir, "ebigent.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `entry = "./cmd/pong"`) {
		t.Errorf("ebigent.toml does not declare the detected entry:\n%s", body)
	}
}

// A module with nothing to run cannot be configured: buildconf requires
// a target, so every verb would refuse what init had just written.
func TestAdoptingAModuleWithNoEntryPointIsRefused(t *testing.T) {
	s := spec(t, "solo", 0, false)
	s.Adopt = true
	if err := s.Validate(); err == nil {
		t.Fatal("a module with no main package should be refused")
	} else if !strings.Contains(err.Error(), "no main package") {
		t.Errorf("the error does not say what is missing: %v", err)
	}
}

// DetectTargets reads the kind off the import graph rather than the
// directory name, which is the only evidence an adopted repository
// actually carries.
func TestDetectTargetsReadsTheImportGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	s := spec(t, "solo", 0, false)
	generateAndTidy(t, s)

	targets, err := scaffold.DetectTargets(s.Dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, target := range targets {
		kinds[target.Entry()] = target.Kind
	}
	// The playable entry links the engine; the headless one must not,
	// and that difference is rule:engine-import-confined-to-client-entry
	// read back out of a built module.
	if kinds["./cmd/mygame"] != "client" {
		t.Errorf("the rendering entry should be a client, got %q", kinds["./cmd/mygame"])
	}
	if kinds["./cmd/simulation"] != "simulation" {
		t.Errorf("the headless entry should be a simulation, got %q", kinds["./cmd/simulation"])
	}
	// The distillation entry is a tool init wrote, not an artifact the
	// project ships, so declaring it would put it in front of build.
	if _, listed := kinds[scaffold.DistillEntry]; listed {
		t.Errorf("%s should not be declared as a build target: %v", scaffold.DistillEntry, kinds)
	}
}

// DistillEntry repeats the behavior.distill default because a struct tag
// cannot name a constant. This is what keeps the two honest, so a
// project that never edits its configuration still distills.
func TestDistillEntryMatchesTheConfigDefault(t *testing.T) {
	field, ok := reflect.TypeFor[buildconf.Behavior]().FieldByName("Distill")
	if !ok {
		t.Fatal("buildconf.Behavior has no Distill field")
	}
	if got := field.Tag.Get("default"); got != scaffold.DistillEntry {
		t.Errorf("behavior.distill defaults to %q but init writes %q", got, scaffold.DistillEntry)
	}
}
