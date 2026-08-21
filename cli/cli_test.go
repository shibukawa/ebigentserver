package cli_test

import (
	"bytes"
	"os"
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
		"--shape", "solo",
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
}

// A second init must not overwrite a live project.
func TestInitRefusesAnExistingProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ebigent.toml"), []byte("[project]\nmodule = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, "", "init", dir, "--yes",
		"--module", "example.com/probe", "--name", "probe",
		"--shape", "solo", "--skip_tidy")
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
func TestPendingVerbNamesWhatItWaitsOn(t *testing.T) {
	code, _, errOut := run(t, t.TempDir(), "dev")
	if code == 0 {
		t.Fatal("dev is not implemented yet and should fail")
	}
	if !strings.Contains(errOut, "not implemented yet") {
		t.Errorf("stderr = %q", errOut)
	}
}
