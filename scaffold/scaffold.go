// Package scaffold writes a new game project: flow:project-init steps 4
// through 6, the part that puts files on disk.
//
// What it writes has to compile and run before anything is hand edited
// (requirement:project-scaffolding). The game it generates is a
// deliberate placeholder — small enough that nobody mistakes it for
// their game, complete enough that the session loop, the validator, the
// observation projection, and the evaluation signal are all already
// wired to replace one piece at a time.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/shibukawa/ebigentserver/skills"
)

//go:embed templates
var templates embed.FS

// FrameworkModule is the import path a generated project depends on.
const FrameworkModule = "github.com/shibukawa/ebigentserver"

// TargetKinds are the concept:build-target rows init can generate an
// entry point for, in the order a wizard offers them.
var TargetKinds = []string{"client", "listen", "dedicated", "simulation"}

// Topologies and SyncModes mirror the run configuration allowlists; the
// wizard offers these and writes the choice into ebigent.toml.
var (
	Topologies = []string{"standalone", "listen", "dedicated", "p2p"}
	SyncModes  = []string{"delay", "rollback", "server_authoritative", "hybrid"}
)

// topologyTargets is which targets a topology can actually use, and so
// which ones the wizard offers once the topology is chosen.
var topologyTargets = map[string][]string{
	"standalone": {"client", "simulation"},
	"listen":     {"listen", "client", "simulation"},
	"dedicated":  {"dedicated", "client", "simulation"},
	"p2p":        {"client", "simulation"},
}

// topologySync is which synchronization modes each topology supports, so
// step 2 of flow:project-init offers only reachable combinations.
var topologySync = map[string][]string{
	"standalone": {"server_authoritative"},
	"listen":     {"server_authoritative", "delay", "rollback", "hybrid"},
	"dedicated":  {"server_authoritative", "delay", "hybrid"},
	"p2p":        {"delay", "rollback"},
}

// TargetsFor reports the build targets a topology can use.
func TargetsFor(topology string) []string { return topologyTargets[topology] }

// SyncModesFor reports the synchronization modes a topology supports.
func SyncModesFor(topology string) []string { return topologySync[topology] }

// Target is one generated entry point.
type Target struct {
	// Name is the directory under cmd and the target name in
	// ebigent.toml.
	Name string
	// Kind is the concept:build-target row.
	Kind string
}

// Entry is the main package path of this target.
func (t Target) Entry() string { return "./cmd/" + t.Name }

// Dev reports whether this target links api:dev-debug-endpoint. Only the
// simulation and client rows are dev-only today; nothing generated here
// links the endpoint yet, so it is always false and
// rule:debug-endpoint-excluded-from-release has nothing to enforce until
// the endpoint exists.
func (t Target) Dev() bool { return false }

// Spec is one project to write.
type Spec struct {
	// Dir is the project root, created if missing.
	Dir string
	// Module is the go module path.
	Module string
	// Name identifies the game in session IDs and the chip library.
	Name string
	// Topology is the concept:execution-topology chosen in step 1.
	Topology string
	// SyncMode is the concept:synchronization-mode chosen in step 2.
	SyncMode string
	// Targets are the entry points chosen in step 3.
	Targets []Target
	// FrameworkPath, when set, adds a replace directive pointing at a
	// local checkout. It is what lets a project build against
	// unreleased framework code, and what the scaffold's own test uses.
	FrameworkPath string
	// GoVersion pins the toolchain of the generated project.
	GoVersion string
}

// Validate rejects a spec that could not produce a working project.
func (s *Spec) Validate() error {
	var errs []error
	if s.Dir == "" {
		errs = append(errs, errors.New("Dir is required"))
	}
	if s.Module == "" {
		errs = append(errs, errors.New("Module is required"))
	}
	if s.Name == "" {
		errs = append(errs, errors.New("Name is required"))
	}
	if !slices.Contains(Topologies, s.Topology) {
		errs = append(errs, fmt.Errorf("topology %q is not one of %v", s.Topology, Topologies))
	}
	if !slices.Contains(SyncModes, s.SyncMode) {
		errs = append(errs, fmt.Errorf("sync mode %q is not one of %v", s.SyncMode, SyncModes))
	}
	if allowed, ok := topologySync[s.Topology]; ok && !slices.Contains(allowed, s.SyncMode) {
		errs = append(errs, fmt.Errorf("topology %q does not support sync mode %q; it supports %v", s.Topology, s.SyncMode, allowed))
	}
	if len(s.Targets) == 0 {
		errs = append(errs, errors.New("declare at least one build target"))
	}
	seen := map[string]bool{}
	for _, t := range s.Targets {
		if !slices.Contains(TargetKinds, t.Kind) {
			errs = append(errs, fmt.Errorf("target kind %q is not one of %v", t.Kind, TargetKinds))
		}
		if allowed, ok := topologyTargets[s.Topology]; ok && !slices.Contains(allowed, t.Kind) {
			errs = append(errs, fmt.Errorf("topology %q cannot use a %q target; it can use %v", s.Topology, t.Kind, allowed))
		}
		if t.Name == "" {
			errs = append(errs, errors.New("every target needs a name"))
		}
		if seen[t.Name] {
			errs = append(errs, fmt.Errorf("two targets are both named %q", t.Name))
		}
		seen[t.Name] = true
	}
	if len(errs) > 0 {
		return fmt.Errorf("scaffold: invalid spec: %w", errors.Join(errs...))
	}
	return nil
}

// DevTarget is the target ebigent dev runs by default: the first one a
// developer would want in front of them, which is a playable client
// where there is one.
func (s *Spec) DevTarget() string {
	for _, want := range []string{"listen", "client", "dedicated", "simulation"} {
		for _, t := range s.Targets {
			if t.Kind == want {
				return t.Name
			}
		}
	}
	return ""
}

// Framework reports the framework module path for templates.
func (s *Spec) Framework() string { return FrameworkModule }

// Result lists what a Generate run wrote, relative to the project root.
type Result struct {
	Files []string
}

// Generate writes the project. It refuses to touch a directory that
// already holds any file it would write, so a mistaken second init in a
// live project reports a conflict instead of overwriting work.
func Generate(spec *Spec) (*Result, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	files, err := render(spec)
	if err != nil {
		return nil, err
	}
	var clashes []string
	for name := range files {
		if _, err := os.Stat(filepath.Join(spec.Dir, filepath.FromSlash(name))); err == nil {
			clashes = append(clashes, name)
		}
	}
	if len(clashes) > 0 {
		slices.Sort(clashes)
		return nil, fmt.Errorf("scaffold: %s already holds %s; init will not overwrite", spec.Dir, strings.Join(clashes, ", "))
	}

	written := make([]string, 0, len(files))
	for name, body := range files {
		full := filepath.Join(spec.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, err
		}
		mode := fs.FileMode(0o644)
		if strings.HasSuffix(name, ".py") {
			mode = 0o755
		}
		if err := os.WriteFile(full, body, mode); err != nil {
			return nil, err
		}
		written = append(written, name)
	}
	slices.Sort(written)
	return &Result{Files: written}, nil
}

// render builds every file in memory first, so a template error leaves
// nothing half written on disk.
func render(spec *Spec) (map[string][]byte, error) {
	out := map[string][]byte{}

	fixed := map[string]string{
		"ebigent.toml":      "ebigent.toml.tmpl",
		"go.mod":            "go.mod.tmpl",
		".gitignore":        "gitignore.tmpl",
		"README.md":         "README.md.tmpl",
		"game/game.go":      "game.go.tmpl",
		"game/game_test.go": "game_test.go.tmpl",
	}
	for name, tmpl := range fixed {
		body, err := execute(tmpl, spec)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(name, ".go") {
			if body, err = gofmtSource(name, body); err != nil {
				return nil, err
			}
		}
		out[name] = body
	}

	for _, t := range spec.Targets {
		data := struct {
			*Spec
			Target Target
		}{spec, t}
		body, err := execute("main_"+t.Kind+".go.tmpl", data)
		if err != nil {
			return nil, err
		}
		name := path.Join("cmd", t.Name, "main.go")
		if body, err = gofmtSource(name, body); err != nil {
			return nil, err
		}
		out[name] = body
	}

	// decision:ai-pipeline-always-scaffolded: the corpus root, the chip
	// library, and the analysis skill exist before there is any data,
	// because recording has to be in place first.
	lib, err := execute("chips.json.tmpl", spec)
	if err != nil {
		return nil, err
	}
	out["behavior/chips.json"] = lib
	out["corpus/.gitkeep"] = []byte{}

	if err := fs.WalkDir(skills.BehaviorAnalyze, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := skills.BehaviorAnalyze.ReadFile(p)
		if err != nil {
			return err
		}
		out[path.Join("skills", p)] = body
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func execute(name string, data any) ([]byte, error) {
	t, err := template.New(name).ParseFS(templates, "templates/"+name)
	if err != nil {
		return nil, fmt.Errorf("scaffold: template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("scaffold: template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// gofmtSource formats generated Go and turns a template that produced
// invalid syntax into an error naming the file, which is far easier to
// act on than a compile error in a directory that was just created.
func gofmtSource(name string, src []byte) ([]byte, error) {
	formatted, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("scaffold: generated %s does not parse: %w", name, err)
	}
	return formatted, nil
}

// Tidy runs go mod tidy in the project, which is step 7 of
// flow:project-init: the generated go.mod names the framework at a
// placeholder version and tidy resolves it, writing go.sum.
//
// It is separate from Generate so that writing files never depends on a
// toolchain or a network, and so a caller can report the two failures
// differently — a template bug and an unreachable module are not the
// same problem.
func Tidy(dir string, env []string) error { return goRun(dir, env, "mod", "tidy") }

// BuildAll compiles every package, which is step 8: init reports whether
// what it just wrote actually builds rather than leaving the developer
// to find out.
func BuildAll(dir string, env []string) error { return goRun(dir, env, "build", "./...") }

func goRun(dir string, env []string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scaffold: go %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
