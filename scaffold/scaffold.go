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

// SyncModes mirrors the run configuration allowlist.
var SyncModes = []string{"delay", "rollback", "server_authoritative", "hybrid"}

// Tuning is the timing declaration written into the generated rules.
type Tuning struct {
	TickRate      int
	SendRate      int
	SnapshotEvery int
	HistoryDepth  int
}

// Styles is how a project is played, which is the first thing a developer
// can answer without knowing anything about transports.
//
// duo is its own style rather than "multi with two seats" because two is
// where a peer link is genuinely one hop: the host is the other player.
// Past two, every exchange goes through a host either way, so the one-hop
// netcodes stop being reachable (concept:deployment-combination).
var Styles = []string{"solo", "duo", "multi"}

// Agents are the agentic environments init knows a skill location for.
// Which one a developer uses is a fact about their tooling rather than a
// project setting, so it decides a path at generation time and is never
// read again — the environment finds the skill by its own convention.
var Agents = []string{"claude", "other"}

// agentSkillDir is where each environment looks for project skills.
var agentSkillDir = map[string]string{
	"claude": ".claude/skills",
	"other":  ".agents/skills",
}

// SkillDirFor reports the skill directory an environment reads.
func SkillDirFor(agent string) string { return agentSkillDir[agent] }

// Reaches is where the traffic goes. It is derived from the seat count
// rather than asked: solo has nowhere to go, and past two seats the
// choice between a peer host and a dedicated one stops changing the
// generated code — it becomes the data:run-config topology value, which a
// project flips without regenerating anything.
var Reaches = []string{"local", "linked"}

// reachTargets is the entry point set each reach needs. Local play needs
// no server whatever the seat count or the camera. Peer and server play
// generate the same set, because the host is orthogonal and lives behind
// a build tag — what differs is which form the project expects to run.
var reachTargets = map[string][]Target{
	"local": {
		{Name: "game", Kind: "client"},
		{Name: "simulation", Kind: "simulation"},
	},
	"linked": {
		{Name: "client", Kind: "client"},
		{Name: "server", Kind: "server"},
		{Name: "simulation", Kind: "simulation"},
	},
}

// ReachFor derives the reach from the seat count.
func ReachFor(seats int) string {
	if seats <= 1 {
		return "local"
	}
	return "linked"
}

// TopologyForStyle is the data:run-config topology a project starts at.
//
// A starting value, not a constraint. Every seat count can run under
// either host: four rows of concept:deployment-combination carry a
// playing host at "2 or many" seats, and a browser hosting a party over
// WebRTC with no backend at all is concept:static-host-mode. The
// generated config says so and both builds are produced, so flipping this
// is a config edit rather than a regeneration.
//
// duo starts at a playing host because one hop is the reason it exists as
// a style. multi starts at a dedicated one whatever its seat count: a
// project that picks multi at two seats is saying latency is not what
// decides the game, so the trustworthy host is the better starting point.
// Both remain starting values, not requirements.
func TopologyForStyle(style string) string {
	switch style {
	case "solo":
		return "standalone"
	case "duo":
		return "listen"
	default:
		return "dedicated"
	}
}

// tuningForSeats is the data:session-tuning-profile a project starts at.
// The framework ships no defaults (decision:no-framework-tuning-defaults);
// these are a starting declaration. Past two seats a hop through a host
// is unavoidable, so that tier sends less often and leans on
// concept:client-prediction instead.
func tuningForSeats(seats int) Tuning {
	switch {
	case seats <= 2:
		return Tuning{TickRate: 60, SendRate: 60, SnapshotEvery: 120, HistoryDepth: 12}
	default:
		return Tuning{TickRate: 60, SendRate: 30, SnapshotEvery: 120, HistoryDepth: 12}
	}
}

// SyncDefaultFor is the synchronization mode a style usually wants. Two
// players reach each other in one hop, which is what term:rollback
// assumes; past two, every exchange is two hops and authority plus
// concept:client-prediction is what is left.
func SyncDefaultFor(style string) string {
	if style == "duo" {
		return "rollback"
	}
	return "server_authoritative"
}

// TargetsFor reports the entry points a seat count generates.
func TargetsFor(seats int) []Target { return reachTargets[ReachFor(seats)] }

// SyncModesFor reports the synchronization modes a seat count can run.
//
// Solo has none: synchronization keeps sessions consistent across a link,
// and there is no link. Two seats can reach a peer directly in one hop,
// which is what term:rollback and term:delay-buffering assume. Past two,
// every exchange is two hops whichever host is chosen, so those two stop
// being reachable and authority is what is left
// (concept:deployment-combination).
func SyncModesFor(seats int) []string {
	switch {
	case seats <= 1:
		return nil
	case seats == 2:
		return []string{"rollback", "delay", "hybrid", "server_authoritative"}
	default:
		return []string{"server_authoritative", "hybrid", "delay"}
	}
}

// NeedsUnreliable reports whether a reach wants a datagram channel.
//
// Any link does, even for a game whose turns are slow: cursors, look
// direction, and ping markers are data:presence-message, which travels at
// its own rate and is superseded rather than retransmitted
// (rule:presence-superseded-not-retransmitted). A turn-based game with
// live cursors is realtime in the transport even though its simulation
// is not.
func NeedsUnreliable(seats int) bool { return seats > 1 }

// Target is one generated entry point.
type Target struct {
	// Name is the directory under cmd and the target name in
	// ebigent.toml.
	Name string
	// Kind is the concept:build-target row: client, server, or
	// simulation. A server carries both linkage forms in one directory
	// and picks between them with a build tag, so listen and dedicated
	// are not separate kinds.
	Kind string
}

// Entry is the main package path of this target.
func (t Target) Entry() string { return "./cmd/" + t.Name }

// Tagged reports whether this target has a second linkage form selected
// by a build tag (rule:build-tag-only-for-linkage). Only a server does:
// built plain it is headless and never links the engine, built with the
// listen tag it also seats the local player.
func (t Target) Tagged() bool { return t.Kind == "server" }

// ConfigKind maps a generated target onto the data:build-config kind,
// which still names the two server forms separately because a built
// artifact is one or the other.
func (t Target) ConfigKind() string {
	if t.Kind == "server" {
		return "dedicated"
	}
	return t.Kind
}

// Spec is one project to write.
type Spec struct {
	// Dir is the project root, created if missing.
	Dir string
	// Module is the go module path.
	Module string
	// Name identifies the game in session IDs and the chip library.
	Name string
	// Style is how the project is played: solo, duo, or multi.
	Style string
	// Agent is the agentic environment the developer works in, which
	// decides where the analysis skill is written so that environment
	// finds it without being told.
	Agent string
	// SharedScreen means every seat reads the same screen content, which
	// is the shared arrangement of concept:view-arrangement.
	//
	// It is about the content, not the machine. Two players at one
	// keyboard and two on separate machines both read the same stage in a
	// fighting game; both are this answer. What it excludes is a view per
	// seat, which locally means split viewports and remotely means each
	// window showing something different.
	//
	// What it decides in generated code: whether the client seats every
	// slot in front of one view, and whether concept:visibility-scope can
	// be anything but global.
	SharedScreen bool
	// Seats is how many concept:player-slot entries the rules declare.
	// A number rather than a category: two seats and twenty generate the
	// same wiring.
	Seats int
	// SyncMode is the concept:synchronization-mode chosen in step 2.
	SyncMode string
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

	if !slices.Contains(Agents, s.Agent) {
		errs = append(errs, fmt.Errorf("agent environment %q is not one of %v", s.Agent, Agents))
	}
	if !slices.Contains(Styles, s.Style) {
		errs = append(errs, fmt.Errorf("play style %q is not one of %v", s.Style, Styles))
	}
	if want, ok := seatsForStyle(s.Style, s.Seats); ok && s.Seats != want {
		errs = append(errs, fmt.Errorf("the %q style declares %d seats, not %d", s.Style, want, s.Seats))
	}
	if s.Style == "solo" && s.SharedScreen {
		errs = append(errs, errors.New("one player has nobody to share a screen with"))
	}
	if s.Seats < 1 {
		errs = append(errs, fmt.Errorf("a session needs at least one seat, not %d", s.Seats))
	}

	if s.Seats > 8 {
		errs = append(errs, fmt.Errorf("%d seats is past what the generated placeholder renders; declare the slots by hand instead", s.Seats))
	}
	switch modes := SyncModesFor(s.Seats); {
	case len(modes) == 0 && s.SyncMode != "":
		errs = append(errs, errors.New("one seat has no synchronization mode to set: there is no link to keep consistent"))
	case len(modes) > 0 && !slices.Contains(modes, s.SyncMode):
		errs = append(errs, fmt.Errorf("sync mode %q is not one of %v", s.SyncMode, modes))
	}
	if len(errs) > 0 {
		return fmt.Errorf("scaffold: invalid spec: %w", errors.Join(errs...))
	}
	return nil
}

// Targets is the entry point set this process boundary generates.
func (s *Spec) Targets() []Target { return reachTargets[ReachFor(s.Seats)] }

// Topology is the data:run-config topology this boundary starts at.
func (s *Spec) Topology() string { return TopologyForStyle(s.Style) }

// Tuning is the timing declaration for this spec's pace.
func (s *Spec) Tuning() Tuning { return tuningForSeats(s.Seats) }

// RequireUnreliable reports whether the generated run config asks for a
// datagram channel.
func (s *Spec) RequireUnreliable() bool { return NeedsUnreliable(s.Seats) }

// seatsForStyle reports the seat count a style fixes, and whether it
// fixes one at all — multi takes its count from the developer.
func seatsForStyle(style string, seats int) (int, bool) {
	switch style {
	case "solo":
		return 1, true
	case "duo":
		return 2, true
	default:
		return seats, false
	}
}

// SeatsForStyle reports the seat count a style implies, using fallback
// for the style that does not fix one.
func SeatsForStyle(style string, fallback int) int {
	if n, ok := seatsForStyle(style, fallback); ok {
		return n
	}
	return fallback
}

// SlotNames renders the generated slot identifiers, so the rules template
// can declare one constant per seat.
func (s *Spec) SlotNames() []string {
	names := make([]string, 0, s.Seats)
	for i := range s.Seats {
		names = append(names, fmt.Sprintf("Slot%d", i+1))
	}
	return names
}

// DevTarget is the target ebigent dev runs by default: the one a
// developer would want in front of them, which is the playable client.
func (s *Spec) DevTarget() string {
	for _, t := range s.Targets() {
		if t.Kind == "client" {
			return t.Name
		}
	}
	if ts := s.Targets(); len(ts) > 0 {
		return ts[0].Name
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
		"boundary_test.go":  "boundary_test.go.tmpl",
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

	for _, t := range spec.Targets() {
		data := struct {
			*Spec
			Target Target
		}{spec, t}
		// A server directory holds three files: the shared body and one
		// small file per linkage form, selected by the listen tag.
		files := map[string]string{"main.go": "main_" + t.Kind + ".go.tmpl"}
		if t.Tagged() {
			files["listen.go"] = "server_listen.go.tmpl"
			files["headless.go"] = "server_headless.go.tmpl"
		}
		for base, tmpl := range files {
			body, err := execute(tmpl, data)
			if err != nil {
				return nil, err
			}
			name := path.Join("cmd", t.Name, base)
			if body, err = gofmtSource(name, body); err != nil {
				return nil, err
			}
			out[name] = body
		}
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
		out[path.Join(agentSkillDir[spec.Agent], p)] = body
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func execute(name string, data any) ([]byte, error) {
	t, err := template.New(name).Funcs(template.FuncMap{
		"addOne": func(i int) int { return i + 1 },
	}).ParseFS(templates, "templates/"+name)
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
