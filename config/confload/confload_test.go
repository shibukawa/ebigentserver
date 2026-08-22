package confload_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/config/buildconf"
	"github.com/shibukawa/ebigentserver/config/confload"
	"github.com/shibukawa/ebigentserver/config/runconf"
	"github.com/shibukawa/tinybind-go/configbind"
)

// wholeFile carries both readers' sections, which is the point of
// decision:one-config-file-many-sections: one file, and a prefix decides
// which struct binds a key.
const wholeFile = `
[project]
module = "example.com/game"

[build]

[[build.target]]
name = "server"
kind = "dedicated"
entry = "./cmd/server"

[dev]
console = "127.0.0.1:9000"

[protocol]
shape = "duo"
devices = ["keyboard"]

[protocol.seats]
count = 2

[run]
topology = "dedicated"
listen = "0.0.0.0:4433"
tuning.baseline = "confirmed_only"
tuning.tick_rate = 30
`

func writeProject(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, confload.FileName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// reset clears Bind registrations, which are process state shared by
// every test in this binary.
func reset(t *testing.T) {
	t.Helper()
	configbind.ResetTargets()
	t.Cleanup(configbind.ResetTargets)
}

func TestArtifactReadsRunSectionsAndIgnoresToolchainOnes(t *testing.T) {
	reset(t)
	dir := writeProject(t, wholeFile)
	run := runconf.Bind()

	if _, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Args: []string{}, Environ: []string{},
	}); err != nil {
		t.Fatalf("load: %v", err)
	}

	if run.Topology != "dedicated" {
		t.Errorf("topology = %q, want dedicated", run.Topology)
	}
	if run.Listen != "0.0.0.0:4433" {
		t.Errorf("listen = %q", run.Listen)
	}
	if run.Tuning.Baseline != "confirmed_only" {
		t.Errorf("tuning.baseline = %q", run.Tuning.Baseline)
	}
	if run.Tuning.TickRate != 30 {
		t.Errorf("tuning.tick_rate = %d", run.Tuning.TickRate)
	}
	// Defaults still apply to keys the file never mentions.
	if run.Tuning.Ack != "piggyback_only" {
		t.Errorf("tuning.ack = %q, want the default", run.Tuning.Ack)
	}
}

func TestToolReadsToolchainSectionsAndIgnoresRunOnes(t *testing.T) {
	reset(t)
	dir := writeProject(t, wholeFile)
	cfg := buildconf.Bind()

	if _, err := confload.Load(confload.Options{
		Owned: buildconf.Prefixes(), StartDir: dir,
		Args: []string{}, Environ: []string{},
	}); err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Project.Module != "example.com/game" {
		t.Errorf("project.module = %q", cfg.Project.Module)
	}
	if len(cfg.Build.Target) != 1 || cfg.Build.Target[0].Entry != "./cmd/server" {
		t.Fatalf("targets = %+v", cfg.Build.Target)
	}
	if cfg.Dev.Console != "127.0.0.1:9000" {
		t.Errorf("dev.console = %q", cfg.Dev.Console)
	}
}

func TestPrecedenceCLIOverEnvOverFileOverDefault(t *testing.T) {
	reset(t)
	dir := writeProject(t, "[run]\ntopology = \"listen\"\nlisten = \"from-file:1\"\n")
	run := runconf.Bind()

	if _, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Environ: []string{"RUN_LISTEN=from-env:2"},
		Args:    []string{"--run-listen", "from-cli:3"},
	}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if run.Listen != "from-cli:3" {
		t.Errorf("listen = %q, want the CLI value to win", run.Listen)
	}
	if run.Topology != "listen" {
		t.Errorf("topology = %q, want the file value where nothing outranks it", run.Topology)
	}
	if run.Tuning.Ack != "piggyback_only" {
		t.Errorf("tuning.ack = %q, want the default", run.Tuning.Ack)
	}
}

func TestEnvBeatsFileAndLosesToCLI(t *testing.T) {
	reset(t)
	dir := writeProject(t, "[run]\nlisten = \"from-file:1\"\n")
	run := runconf.Bind()

	if _, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Environ: []string{"RUN_LISTEN=from-env:2"},
		Args:    []string{},
	}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if run.Listen != "from-env:2" {
		t.Errorf("listen = %q, want the environment value", run.Listen)
	}
}

func TestStrayKeyInOwnedSectionIsRejected(t *testing.T) {
	reset(t)
	dir := writeProject(t, "[run]\ntopolgy = \"dedicated\"\n")
	runconf.Bind()

	_, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Args: []string{}, Environ: []string{},
	})
	var stray *confload.StrayKeyError
	if !asStray(err, &stray) {
		t.Fatalf("err = %v, want a StrayKeyError", err)
	}
	if len(stray.Keys) != 1 || stray.Keys[0] != "run.topolgy" {
		t.Errorf("stray keys = %v", stray.Keys)
	}
}

func TestStrayKeyOutsideOwnedSectionIsIgnored(t *testing.T) {
	reset(t)
	// The artifact owns run.* only. A typo in a toolchain section is
	// not its business, and objecting to it would break the shared file.
	dir := writeProject(t, "[project]\nmoduel = \"typo\"\n\n[run]\ntopology = \"standalone\"\n")
	run := runconf.Bind()

	if _, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Args: []string{}, Environ: []string{},
	}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if run.Topology != "standalone" {
		t.Errorf("topology = %q", run.Topology)
	}
}

func TestStrayKeyInsideTableArrayElementIsRejected(t *testing.T) {
	reset(t)
	dir := writeProject(t, "[build]\n\n[[build.target]]\nname = \"server\"\nnmae = \"typo\"\n")
	buildconf.Bind()

	_, err := confload.Load(confload.Options{
		Owned: buildconf.Prefixes(), StartDir: dir,
		Args: []string{}, Environ: []string{},
	})
	var stray *confload.StrayKeyError
	if !asStray(err, &stray) {
		t.Fatalf("err = %v, want a StrayKeyError", err)
	}
	if len(stray.Keys) != 1 || stray.Keys[0] != "build.target[0].nmae" {
		t.Errorf("stray keys = %v", stray.Keys)
	}
}

// The enum struct tag is documentation in tinybind-go v0.5.17: it reaches
// neither the generated code nor the loader, so an unlisted value binds
// happily. Validate is the only thing standing between a typo and a
// silently wrong topology, which is why Load runs it.
func TestValueOutsideTheAllowlistIsRejectedByValidate(t *testing.T) {
	reset(t)
	dir := writeProject(t, "[run]\ntopology = \"peer2peer\"\n")
	run := runconf.Bind()

	_, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Args: []string{}, Environ: []string{},
		Validate: []func() error{func() error { return run.Validate() }},
	})
	if err == nil {
		t.Fatal("want an error for a value outside the allowlist")
	}
	if !strings.Contains(err.Error(), "run.topology") {
		t.Errorf("err = %v, want it to name run.topology", err)
	}
}

func TestLoadItselfDoesNotEnforceTheEnumTag(t *testing.T) {
	reset(t)
	dir := writeProject(t, "[run]\ntopology = \"peer2peer\"\n")
	run := runconf.Bind()

	// No Validate hook: this documents the gap the hook exists to close.
	if _, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Args: []string{}, Environ: []string{},
	}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if run.Topology != "peer2peer" {
		t.Errorf("topology = %q; the enum tag was expected to be inert at load", run.Topology)
	}
}

func TestFindProjectRootWalksUpward(t *testing.T) {
	dir := writeProject(t, "[project]\nmodule = \"example.com/game\"\n")
	deep := filepath.Join(dir, "cmd", "server", "internal")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := confload.FindProjectRoot(deep)
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	// t.TempDir may hand back a symlinked path on darwin; compare the
	// resolved forms so the walk is what is under test, not /var vs
	// /private/var.
	if resolve(t, got) != resolve(t, dir) {
		t.Errorf("root = %q, want %q", got, dir)
	}
}

func TestMissingProjectIsAnErrorUnlessAllowed(t *testing.T) {
	reset(t)
	empty := t.TempDir()
	runconf.Bind()
	if _, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: empty,
		Args: []string{}, Environ: []string{},
	}); err == nil {
		t.Fatal("want ErrNoProject outside a project")
	}

	configbind.ResetTargets()
	run := runconf.Bind()
	if _, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: empty, AllowMissingProject: true,
		Args: []string{}, Environ: []string{},
	}); err != nil {
		t.Fatalf("AllowMissingProject: %v", err)
	}
	if run.Topology != "standalone" {
		t.Errorf("topology = %q, want the default outside a project", run.Topology)
	}
}

func TestDeclaredKeysCoverNestedAndElementFields(t *testing.T) {
	declared, err := confload.DeclaredKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"run.topology",
		"run.tuning.baseline",      // nested struct, dotted under its table
		"protocol.team",            // the array itself
		"protocol.team[].occupant", // an element field
		"build.target[].goos",      // a key tag renaming an element field
		"project.go",               // a key tag renaming a scalar
		"behavior.library",
	} {
		if !declared[key] {
			t.Errorf("declared keys are missing %q", key)
		}
	}
	if declared["run.topolgy"] {
		t.Error("a key nobody declared is present")
	}
}

func TestProvenanceReportsTheWinningLayer(t *testing.T) {
	reset(t)
	dir := writeProject(t, "[run]\nlisten = \"from-file:1\"\n")
	runconf.Bind()

	res, err := confload.Load(confload.Options{
		Owned: runconf.Prefixes(), StartDir: dir,
		Environ: []string{"RUN_TOPOLOGY=listen"},
		Args:    []string{},
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var sb strings.Builder
	if err := confload.WriteProvenance(&sb, res); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	for _, want := range []string{
		"run.topology = listen (env)",
		"run.listen = from-file:1 (file_toml)",
		"run.tuning.ack = piggyback_only (default)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("provenance is missing %q; got:\n%s", want, out)
		}
	}
}

func asStray(err error, target **confload.StrayKeyError) bool {
	for err != nil {
		if s, ok := err.(*confload.StrayKeyError); ok {
			*target = s
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

func resolve(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return r
}
