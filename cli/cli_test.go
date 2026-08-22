package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/cli"
	"github.com/shibukawa/tinybind-go/configbind"
)

// run drives the real entry point. Subcommand selection reads os.Args, so
// the arguments go there rather than into a parameter, and Bind targets
// are process state that has to be cleared between invocations.
func run(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)

	oldArgs, oldWd := os.Args, mustGetwd(t)
	os.Args = append([]string{"ebigent"}, args...)
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		os.Args = oldArgs
		_ = os.Chdir(oldWd)
	})

	var out, errOut bytes.Buffer
	code = cli.Run(&out, &errOut)
	return code, out.String(), errOut.String()
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

func frameworkRoot(t *testing.T) string {
	t.Helper()
	return filepath.Dir(mustGetwd(t))
}

// init runs unattended when every answer is an option, which is what
// flow:project-init promises and what makes this test possible at all.
func TestInitNonInteractiveProducesAWorkingProject(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	code, out, errOut := run(t, "", "init", dir,
		"--yes", "--module", "example.com/probe", "--name", "probe",
		"--style", "solo",
		"--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	for _, want := range []string{"ebigent.toml", "game/game.go", "cmd/simulation/main.go"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("init did not write %s", want)
		}
	}
	if !strings.Contains(out, "probe is ready") {
		t.Errorf("stdout = %q", out)
	}

	// Every key the scaffold writes has to reach the overlay, or the
	// template is naming something no struct binds.
	code, out, errOut = run(t, dir, "config", "show")
	if code != 0 {
		t.Fatalf("config show in a fresh project: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	for _, want := range []string{"protocol.package", "protocol.shape", "protocol.seats.count"} {
		if !strings.Contains(out, want) {
			t.Errorf("the effective configuration does not report %s:\n%s", want, out)
		}
	}

	// And it has to satisfy the validation a project-scoped verb runs
	// before it touches anything. config show skips that check, so on its
	// own it would accept a template that writes a file the next command
	// rejects — which is exactly what adding a required section risks.
	code, out, errOut = run(t, dir, "build", "simulation")
	if code != 0 {
		t.Fatalf("build in a fresh project: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
}

// A second init must not overwrite a live project.
func TestInitRefusesAnExistingProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ebigent.toml"), []byte("[project]\nmodule = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, "", "init", dir, "--yes",
		"--module", "example.com/probe", "--name", "probe",
		"--style", "solo", "--skip_tidy")
	if code == 0 {
		t.Fatal("a second init should fail")
	}
	if !strings.Contains(errOut, "will not overwrite") {
		t.Errorf("stderr = %q", errOut)
	}
}

// A verb that needs a project says which file is missing and what to do,
// rather than reporting a condition.
func TestVerbOutsideAProjectExplainsItself(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "build")
	if code == 0 {
		t.Fatal("build outside a project should fail")
	}
	if !strings.Contains(errOut, "ebigent.toml") || !strings.Contains(errOut, "ebigent init") {
		t.Errorf("stderr = %q, want it to name the file and the fix", errOut)
	}
}

func TestHelpExitsZeroAndListsVerbs(t *testing.T) {
	code, out, _ := run(t, t.TempDir(), "--help")
	if code != 0 {
		t.Errorf("--help exit = %d, want 0", code)
	}
	for _, verb := range []string{"init", "build", "config", "doctor", "analyze", "merge", "version"} {
		if !strings.Contains(out, verb) {
			t.Errorf("help is missing the %s verb", verb)
		}
	}
}

// A parse failure has to exit non-zero even though printing usage is the
// most useful response: an exit code is what a script reads.
func TestUnknownOptionIsAFailureNotHelp(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "config", "show", "--run-topology", "p2p")
	if code == 0 {
		t.Fatal("an unknown verb option should not exit 0")
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Errorf("stderr = %q, want usage text alongside the failure", errOut)
	}
}

// A configuration key is a global option and belongs before the verb.
func TestConfigOptionBeforeTheVerbWins(t *testing.T) {
	code, out, errOut := run(t, t.TempDir(), "--run-topology", "p2p", "config", "show")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "run.topology = p2p (cli)") {
		t.Errorf("stdout = %q", out)
	}
}

// config scaffold renders the declared shape, which is what a developer
// copies into ebigent.toml before there is a project.
func TestConfigScaffoldRendersEverySection(t *testing.T) {
	code, out, errOut := run(t, t.TempDir(), "config", "scaffold")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, section := range []string{"[project]", "[[build.target]]", "[dev]", "[behavior]", "[run]"} {
		if !strings.Contains(out, section) {
			t.Errorf("scaffold is missing %s", section)
		}
	}
}

// A verb that is declared but not built yet says what it is waiting on,
// which is more useful than an unknown-command error.
// A wizard has to tell "said no" apart from "did not say", which a bool
// flag cannot: --shared_screen=false would still have prompted.
func TestExplicitNoSkipsTheSharedScreenQuestion(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	code, out, errOut := run(t, "", "init", dir,
		"--yes", "--module", "example.com/probe", "--name", "probe",
		"--style", "duo", "--shared_screen", "no",
		"--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if strings.Contains(out, "Do all players read the same screen content") {
		t.Error("an explicit no should not be asked again")
	}
	if strings.Contains(out, "every seat may know") {
		t.Error("the shared-stage warning belongs to a yes")
	}
}

func TestPendingVerbNamesWhatItWaitsOn(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "dev")
	if code == 0 {
		t.Fatal("dev is not implemented yet and should fail")
	}
	if !strings.Contains(errOut, "not implemented yet") {
		t.Errorf("stderr = %q", errOut)
	}
}

// TestGenerateEmitsTheProtocolConstants covers requirement:config-codegen:
// what ebigent.toml settles at build reaches the artifact as Go rather
// than as a startup lookup.
func TestGenerateEmitsTheProtocolConstants(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	code, out, errOut := run(t, "", "init", dir,
		"--yes", "--module", "example.com/squad", "--name", "squad",
		"--style", "multi", "--seats", "4", "--shared_screen", "no",
		"--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("init: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}

	code, out, errOut = run(t, dir, "generate")
	if code != 0 {
		t.Fatalf("generate: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	src, err := os.ReadFile(filepath.Join(dir, "internal", "ebigentgen", "protocol_gen.go"))
	if err != nil {
		t.Fatalf("generate wrote nothing: %v", err)
	}
	for _, want := range []string{
		`const Package = "example.com/squad"`,
		`const Title = "squad"`,
		`Shape    = "multi"`,
		`View     = "per_agent"`,
		`const SeatCount = 4`,
		`const FillUnclaimedSeats = true`,
		`const Teamed = false`,
		`{Slot: 4, Team: "", Occupant: "any"},`,
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("generated source lacks %s:\n%s", want, src)
		}
	}

	// Nothing imports the generated package yet, so building the entry
	// point does not compile it. Checking the text alone would accept a
	// template that emits Go which does not parse.
	compile := exec.Command("go", "build", "./"+GeneratedDirSlash)
	compile.Dir = dir
	if combined, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("the generated package does not compile: %v\n%s", err, combined)
	}

	// Same table, same bytes: a no-op run must not touch the tree, or
	// flow:dev-rebuild-loop rebuilds because it just generated.
	code, out, errOut = run(t, dir, "generate")
	if code != 0 {
		t.Fatalf("second generate: exit %d\nstderr:\n%s", code, errOut)
	}
	if !strings.Contains(out, "unchanged") {
		t.Errorf("a repeated generate did not report the file unchanged: %q", out)
	}
	again, err := os.ReadFile(filepath.Join(dir, "internal", "ebigentgen", "protocol_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(src) {
		t.Error("generate is not idempotent")
	}
}

// TestGenerateFollowsTheTeamDivision covers the part of the seat
// composition that could not be said before the [protocol] table: the
// generated seats carry their team and what may occupy them, so
// api:roster needs no lookup of its own.
func TestGenerateFollowsTheTeamDivision(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	code, out, errOut := run(t, "", "init", dir,
		"--yes", "--module", "example.com/duel", "--name", "duel",
		"--style", "multi", "--seats", "4", "--shared_screen", "yes",
		"--sync", "server_authoritative",
		"--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("init: exit %d\nstderr:\n%s", code, errOut)
	}
	toml := filepath.Join(dir, "ebigent.toml")
	src, err := os.ReadFile(toml)
	if err != nil {
		t.Fatal(err)
	}
	divided := string(src) + `
[[protocol.team]]
name = "red"
seats = 3
occupant = "human"

[[protocol.team]]
name = "blue"
seats = 1
occupant = "bot"
`
	if err := os.WriteFile(toml, []byte(divided), 0o600); err != nil {
		t.Fatal(err)
	}

	code, out, errOut = run(t, dir, "generate")
	if code != 0 {
		t.Fatalf("generate: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	gen, err := os.ReadFile(filepath.Join(dir, "internal", "ebigentgen", "protocol_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`const Teamed = true`,
		`{Slot: 1, Team: "red", Occupant: "human"},`,
		`{Slot: 3, Team: "red", Occupant: "human"},`,
		`{Slot: 4, Team: "blue", Occupant: "bot"},`,
	} {
		if !strings.Contains(string(gen), want) {
			t.Errorf("generated source lacks %s:\n%s", want, gen)
		}
	}

	// Teams that do not account for every seat are refused before they
	// can become a constant.
	short := strings.Replace(divided, "seats = 3", "seats = 2", 1)
	if err := os.WriteFile(toml, []byte(short), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = run(t, dir, "generate")
	if code == 0 {
		t.Fatal("generate accepted a division that leaves a seat on no team")
	}
	if !strings.Contains(errOut, "account for 3 seats") {
		t.Errorf("stderr = %q", errOut)
	}
}

// GeneratedDirSlash is cli.GeneratedDir, spelled here so the test names
// the same directory the verb writes to and cannot drift from it.
const GeneratedDirSlash = cli.GeneratedDir
