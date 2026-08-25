package cli_test

import (
	"bytes"
	"fmt"
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

// A go.mod changes what init is for: the module path and the entry
// points are already decided, so init adds the framework's files beside
// the game instead of writing one.
func TestInitAdoptsAModuleThatAlreadyExists(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := existingModule(t)
	code, out, errOut := run(t, "", "init", dir, "--yes",
		"--style", "solo", "--agent", "other",
		"--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	for _, want := range []string{"ebigent.toml", "behavior/chips.json", "cmd/distill/main.go"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("init did not write %s", want)
		}
	}
	// The point of the mode: the sources that were already there are
	// still the ones there.
	for _, unwanted := range []string{"game/game.go.orig", "README.md", ".gitignore", "cmd/simulation/main.go"} {
		if _, err := os.Stat(filepath.Join(dir, unwanted)); err == nil {
			t.Errorf("init wrote %s into a module that did not ask for one", unwanted)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "game", "game.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "the game that was already here") {
		t.Errorf("init overwrote the module's own game:\n%s", body)
	}
	if !strings.Contains(out, "existing project") {
		t.Errorf("init did not say which mode it was in:\n%s", out)
	}

	// And what it wrote has to satisfy the validation every project-scoped
	// verb runs, which for an adopted module means the entry it detected
	// is one the go tool agrees exists.
	code, out, errOut = run(t, dir, "build", "pong")
	if code != 0 {
		t.Fatalf("build in an adopted project: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
}

// The module path is the one thing an adopted project has already
// settled, so an option that disagrees is a mistake worth naming rather
// than a preference to honour.
func TestInitRefusesAModulePathThatContradictsGoMod(t *testing.T) {
	dir := existingModule(t)
	code, _, errOut := run(t, "", "init", dir, "--yes",
		"--module", "example.com/somethingelse", "--style", "solo", "--skip_tidy")
	if code == 0 {
		t.Fatal("init should refuse to rename an existing module")
	}
	if !strings.Contains(errOut, "already declares module example.com/pong") {
		t.Errorf("stderr = %q", errOut)
	}
}

// existingModule is a repository that has a game and a way to run it,
// and has never heard of ebigent.
func existingModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":           "module example.com/pong\n\ngo 1.25\n",
		"cmd/pong/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"pong\") }\n",
		"game/game.go":     "// Package game is the game that was already here.\npackage game\n\n// Sight is what a seat may see.\ntype Sight struct{ Score int }\n\n// Action is one decision.\ntype Action struct{ Up bool }\n",
	}
	for name, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
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

	// The condition axes reach the artifact the same way, so both ends
	// compare the same set and a refusal can name the term it failed.
	withAxes := divided + `
[[protocol.condition]]
name = "mode"
match = "exact"
values = ["ranked", "casual"]

[[protocol.condition]]
name = "rank"
match = "band"
low = 0
high = 3000
`
	if err := os.WriteFile(toml, []byte(withAxes), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = run(t, dir, "generate")
	if code != 0 {
		t.Fatalf("generate with conditions: exit %d\nstderr:\n%s", code, errOut)
	}
	gen, err = os.ReadFile(filepath.Join(dir, "internal", "ebigentgen", "protocol_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{Name: "mode", Band: false},`,
		`{Name: "rank", Band: true},`,
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

// The agent interface is four methods over two type parameters, and both
// parameters are decided in the rule set assertion rather than in the
// file being written. That is the gap `add agent` closes, so what it
// writes has to compile against the game it read.
func TestAddAgentWritesAgainstTheDeclaredTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	code, out, errOut := run(t, "", "init", dir,
		"--yes", "--module", "example.com/probe", "--name", "probe",
		"--style", "solo", "--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("init: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}

	// Everything after the kind is a question, so an unattended run
	// takes the defaults — and the options are those defaults.
	code, out, errOut = run(t, dir, "add", "agent", "chaser", "--yes")
	if code != 0 {
		t.Fatalf("add agent: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	body, err := os.ReadFile(filepath.Join(dir, "game", "agent_chaser.go"))
	if err != nil {
		t.Fatal(err)
	}
	// The two positions come from `var _ session.TickStageRuleSet[State,
	// Action, Sight]` in the generated rules; nothing here was told them.
	for _, want := range []string{
		"var _ session.Agent[Sight, Action] = (*Chaser)(nil)",
		"func NewChaser(session.SlotID) (string, session.Agent[Sight, Action])",
		`return "chaser", &Chaser{}`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the generated agent does not carry %q:\n%s", want, body)
		}
	}
	// Seating it is one field, and it is the one a developer does not
	// know to look for, so the verb has to name it.
	if !strings.Contains(out, "NewAgent: NewChaser,") {
		t.Errorf("add agent did not say how to seat it:\n%s", out)
	}

	if err := goRunIn(dir, "build", "./..."); err != nil {
		t.Fatalf("the generated agent does not compile: %v", err)
	}

	// An agent is hand written after this point, so a second run naming
	// the same file would throw away the only part worth keeping.
	code, _, errOut = run(t, dir, "add", "agent", "chaser", "--yes")
	if code == 0 {
		t.Fatal("a second add of the same agent should fail")
	}
	if !strings.Contains(errOut, "game/agent_chaser.go already exists") {
		t.Errorf("the refusal does not name the file as a person would type it: %q", errOut)
	}
}

// The kind is a positional, so a typo lands as a kind rather than as an
// unknown verb, and the message has to say what the kinds are.
func TestAddRefusesAKindItCannotWrite(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir)
	code, _, errOut := run(t, dir, "add", "seat", "left")
	if code == 0 {
		t.Fatal("add seat should fail while it is not a kind")
	}
	if !strings.Contains(errOut, "the kinds are agent, stage") {
		t.Errorf("stderr = %q", errOut)
	}
}

// Every answer after the kind is a question, and every option is that
// question's starting value rather than a way past it. A run that
// supplies all three and takes the defaults has to land on all three.
func TestAddOptionsAreTheAnswersTheWizardStartsFrom(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir)
	writeRuleSet(t, filepath.Join(dir, "rules"))

	code, out, errOut := run(t, dir, "add", "agent", "tactic",
		"--type", "Bot", "--file", "bot.go", "--yes")
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	// The questions are reported even when nothing was asked, so an
	// unattended run still says what it chose.
	for _, want := range []string{"Agent name: tactic", "Go type name: Bot", "File name: bot.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("the wizard did not report %q:\n%s", want, out)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "rules", "bot.go"))
	if err != nil {
		t.Fatal(err)
	}
	// The id is the policy and the type is Go's; they are allowed to
	// differ, which is the reason --type exists.
	for _, want := range []string{"type Bot struct", `return "tactic", &Bot{}`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the generated agent does not carry %q:\n%s", want, body)
		}
	}
}

// With nothing supplied there is still an answer for every question, so
// `ebigent add agent` on its own writes something.
func TestAddAgentAnswersEveryQuestionOnItsOwn(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir)
	writeRuleSet(t, filepath.Join(dir, "rules"))

	code, out, errOut := run(t, dir, "add", "agent", "--yes")
	if code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "rules", "agent_bot.go")); err != nil {
		t.Errorf("add agent with no name wrote nothing: %v\n%s", err, out)
	}
	// What it read comes before what it writes, so a wrong sight is
	// caught before the file exists rather than after.
	if !strings.Contains(out, "sight Sight, action Action") {
		t.Errorf("the wizard did not report the types it read:\n%s", out)
	}
}

// The cycle a game developer actually runs is fill a corpus, mine it,
// regenerate. This is the first half, and it has to work on what init
// wrote before anything is hand edited.
func TestSimulateFillsTheCorpusFromWhatInitWrote(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	code, out, errOut := run(t, "", "init", dir,
		"--yes", "--module", "example.com/probe", "--name", "probe",
		"--style", "solo", "--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("init: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}

	code, out, errOut = run(t, dir, "simulate", "--matches", "3", "--seed", "7")
	if code != 0 {
		t.Fatalf("simulate: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	// One episode per match, and the seeds are the recipe: the run
	// reproduces from the first one because each later match adds its
	// index (rule:shared-rng-seed).
	entries, err := os.ReadDir(filepath.Join(dir, "corpus"))
	if err != nil {
		t.Fatal(err)
	}
	var episodes int
	for _, e := range entries {
		if e.IsDir() {
			episodes++
		}
	}
	if episodes != 3 {
		t.Errorf("simulate recorded %d episodes, want 3:\n%s", episodes, out)
	}
	for _, want := range []string{"seed 7", "seed 8", "seed 9"} {
		if !strings.Contains(out, want) {
			t.Errorf("the run does not report %s:\n%s", want, out)
		}
	}

	// And what it recorded has to be what the mining half reads, which
	// is the whole reason the two verbs share a corpus setting.
	code, out, errOut = run(t, dir, "distill")
	if code != 0 {
		t.Fatalf("distill: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if strings.Contains(out, "no decisions under") {
		t.Errorf("distill found nothing simulate had just written:\n%s", out)
	}
}

// The kind decides which entry runs, because concept:build-target
// already separated the headless one from the playing one.
func TestSimulateRefusesAProjectWithNoHeadlessEntry(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir)
	code, _, errOut := run(t, dir, "simulate", "--build=false")
	if code == 0 {
		t.Fatal("simulate should refuse a project with no simulation target")
	}
	if !strings.Contains(errOut, "no target of kind simulation") {
		t.Errorf("stderr = %q", errOut)
	}
}

// A stage is where a game's three types are first named, so nothing can
// be read and everything is declared. What it writes has to compile, and
// its declaration has to be the one `ebigent generate` reads.
func TestAddStageWritesADeclarationGenerateCanRead(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	code, out, errOut := run(t, "", "init", dir,
		"--yes", "--module", "example.com/probe", "--name", "probe",
		"--style", "solo", "--framework_path", frameworkRoot(t))
	if code != 0 {
		t.Fatalf("init: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}

	code, out, errOut = run(t, dir, "add", "stage", "bonus", "--seats", "2", "--yes")
	if code != 0 {
		t.Fatalf("add stage: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	body, err := os.ReadFile(filepath.Join(dir, "bonus", "rules.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"package bonus",
		"const Seats = 2",
		"var _ session.TickStageRuleSet[World, Action, Sight] = RuleSet{}",
		"func (RuleSet) Advance(w *World)",
		"func Config(id string, seed uint64) session.Config[World, Action, Sight]",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the generated stage does not carry %q:\n%s", want, body)
		}
	}
	if err := goRunIn(dir, "build", "./..."); err != nil {
		t.Fatalf("the generated stage does not compile: %v", err)
	}

	// The point of the declaration is that the toolchain reads it. A
	// world or an action the codec generator withdraws would leave the
	// stage unable to cross a link, which is the failure this catches.
	code, out, errOut = run(t, dir, "generate")
	if code != 0 {
		t.Fatalf("generate: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if strings.Contains(out, "no codec for") {
		t.Errorf("generate withdrew a codec the stage needs:\n%s", out)
	}
	for _, want := range []string{"wire_gen.go", "delta_gen.go", "schema_gen.go"} {
		if _, err := os.Stat(filepath.Join(dir, "bonus", want)); err != nil {
			t.Errorf("generate did not write bonus/%s", want)
		}
	}
}

// The two kinds compose in one direction: a stage names the types, and
// only then is there something for an agent to be written against.
func TestAddStageThenAgentClosesTheLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the go toolchain")
	}
	dir := t.TempDir()
	writeProject(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module example.com/probe\n\ngo 1.25\n\nrequire github.com/shibukawa/ebigentserver v0.0.0\n\nreplace github.com/shibukawa/ebigentserver => "+frameworkRoot(t)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Nothing to read yet, and the refusal has to name the way out.
	code, _, errOut := run(t, dir, "add", "agent", "tactic", "--yes")
	if code == 0 {
		t.Fatal("add agent should refuse a project with no rule set")
	}
	if !strings.Contains(errOut, "add stage") {
		t.Errorf("the refusal does not name what writes one: %q", errOut)
	}

	code, out, errOut := run(t, dir, "add", "stage", "duel", "--realtime", "no", "--yes")
	if code != 0 {
		t.Fatalf("add stage: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	// Turn based, so there is no Advance to write.
	body, err := os.ReadFile(filepath.Join(dir, "duel", "rules.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "Advance") {
		t.Errorf("a turn-based stage should not declare Advance:\n%s", body)
	}

	code, out, errOut = run(t, dir, "add", "agent", "tactic", "--yes")
	if code != 0 {
		t.Fatalf("add agent: exit %d\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if !strings.Contains(out, "sight Sight, action Action") {
		t.Errorf("add agent did not read the stage it was just given:\n%s", out)
	}
	// Neither verb resolves modules; both write source into a project
	// that already has a go.mod, so the sum catches up here.
	if err := goRunIn(dir, "mod", "tidy"); err != nil {
		t.Fatal(err)
	}
	if err := goRunIn(dir, "build", "./..."); err != nil {
		t.Fatalf("stage plus agent does not compile: %v", err)
	}
}

// A stage is a package of its own, so writing one into a directory that
// already has rules would be two games sharing a name.
func TestAddStageRefusesAPackageThatIsTaken(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir)
	writeRuleSet(t, filepath.Join(dir, "rules"))
	code, _, errOut := run(t, dir, "add", "stage", "rules", "--yes")
	if code == 0 {
		t.Fatal("add stage should refuse a package that already holds Go source")
	}
	if !strings.Contains(errOut, "already holds Go source") {
		t.Errorf("stderr = %q", errOut)
	}
}

// A repository holding several games cannot be guessed at: writing the
// agent into the wrong one would compile and mean nothing.
func TestAddNamesTheRuleSetsWhenThereAreSeveral(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir)
	for _, name := range []string{"alpha", "beta"} {
		writeRuleSet(t, filepath.Join(dir, name))
	}
	code, _, errOut := run(t, dir, "add", "agent", "tactic", "--yes")
	if code == 0 {
		t.Fatal("add agent should refuse to guess which game")
	}
	for _, want := range []string{"declares 2 rule sets", "no default worth taking", "alpha", "beta"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr does not mention %q: %q", want, errOut)
		}
	}
}

// writeProject is the smallest thing the project-scoped verbs accept: a
// module and a configuration that validates.
func writeProject(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module example.com/probe\n\ngo 1.25\n",
		"ebigent.toml": "[project]\nmodule = \"example.com/probe\"\n\n" +
			"[protocol]\npackage = \"example.com/probe\"\ntitle = \"probe\"\nshape = \"solo\"\ndevices = [\"keyboard\"]\n\n" +
			"[protocol.seats]\ncount = 1\n\n" +
			"[[build.target]]\nname = \"probe\"\nkind = \"client\"\nentry = \".\"\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// writeRuleSet plants one rule set assertion, which is all `add` reads.
func writeRuleSet(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Base(dir)
	body := "package " + pkg + "\n\nimport \"github.com/shibukawa/ebigentserver/session\"\n\n" +
		"type World struct{}\ntype Action struct{}\ntype Sight struct{}\ntype RuleSet struct{}\n\n" +
		"var _ session.StageRuleSet[World, Action, Sight] = RuleSet{}\n"
	if err := os.WriteFile(filepath.Join(dir, pkg+".go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// goRunIn runs the go tool in dir, reporting its output on failure.
func goRunIn(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
