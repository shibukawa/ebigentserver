// Package scaffold writes a new game project: flow:project-init steps 4
// through 6, the part that puts files on disk.
//
// What it writes has to compile and run before anything is hand edited
// (requirement:project-scaffolding). The game it generates is a
// deliberate placeholder — small enough that nobody mistakes it for
// their game, complete enough that the session loop, the validator, the
// sight projection, and the evaluation signal are all already
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
	"strconv"
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

// DistillEntry is the package `ebigent distill` spawns, and the one
// init writes a starting version of.
//
// It repeats the behavior.distill default rather than importing it,
// because a struct tag cannot name a constant and the dependency would
// only run the wrong way. TestDistillEntryMatchesTheConfigDefault holds
// the two together, so a project that never edits behavior.distill
// needs no configuration for distillation to run.
const DistillEntry = "./cmd/distill"

// DedicatedTag makes the game entry a headless server instead of a
// playing one. The tag removes the renderer rather than adding it, so the
// build a developer runs all day is the untagged one and the shipped
// server is the deliberate variant.
const DedicatedTag = "dedicated"

// reachTargets is the entry point set each reach needs. Local play needs
// no server whatever the seat count or the camera. Peer and server play
// generate the same set, because the host is orthogonal and lives behind
// a build tag — what differs is which form the project expects to run.

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

// targetsFor reports the entry points a project generates. The playable
// one carries the game's own name, because that is the binary a developer
// runs and hands to somebody.
func targetsFor(name string, seats int) []Target {
	play := Target{Name: name, Dir: name, Kind: "client"}
	if ReachFor(seats) != "local" {
		// It hosts as well as plays, and the same directory builds a
		// headless server under DedicatedTag.
		play.Kind, play.HasDedicated = "listen", true
	}
	return []Target{play, {Name: "simulation", Dir: "simulation", Kind: "simulation"}}
}

// TargetsFor reports the entry points a seat count generates, for callers
// that only care about the shape.
func TargetsFor(seats int) []Target { return targetsFor("game", seats) }

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
	// Name is the target name in ebigent.toml.
	Name string
	// Dir is the directory under cmd holding it.
	Dir string
	// HasDedicated marks an entry that also builds headless under
	// DedicatedTag.
	HasDedicated bool
	// Kind is the concept:build-target row this artifact occupies.
	Kind string
	// Path is the main package path, for an entry init found rather
	// than wrote. A generated one leaves it empty and takes the cmd
	// layout init also created.
	Path string
}

// Entry is the main package path of this target.
func (t Target) Entry() string {
	if t.Path != "" {
		return t.Path
	}
	return "./cmd/" + t.Dir
}

// Tagged reports whether this target also builds headless under
// DedicatedTag (rule:build-tag-only-for-linkage).
func (t Target) Tagged() bool { return t.HasDedicated }

// DedicatedName is what the headless form is called in ebigent.toml.
func (t Target) DedicatedName() string { return t.Name + "-server" }

// ConfigKind maps a generated target onto the data:build-config kind.
func (t Target) ConfigKind() string { return t.Kind }

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
	// Adopt marks a directory that already holds a go.mod.
	//
	// The placeholder game exists to be replaced, so a module that has
	// its own game has nothing to gain from it and everything to lose:
	// generating game/game.go beside a real one would either collide or
	// look like the project grew a second rule set. So an adopted module
	// gets only what the framework itself owns — the configuration, the
	// corpus root, the chip library, the distillation entry, and the
	// analysis skill — and its sources are left alone.
	Adopt bool
	// Detected are the entry points found in an adopted module, which
	// is where its [[build.target]] blocks come from. In a generated
	// project the entry points are known instead (Targets).
	Detected []Target
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

	if s.Seats > 8 && !s.Adopt {
		errs = append(errs, fmt.Errorf("%d seats is past what the generated placeholder renders; declare the slots by hand instead", s.Seats))
	}
	// A project with no [[build.target]] fails buildconf validation, so
	// every ebigent verb would refuse to run on what init just wrote.
	if s.Adopt && len(s.Detected) == 0 {
		errs = append(errs, errors.New("no main package found in this module; ebigent needs at least one entry point to declare as a build target"))
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

// Targets is the entry point set this project declares: the generated
// one in a new project, and whatever `go list` found in an adopted one.
func (s *Spec) Targets() []Target {
	if s.Adopt {
		return s.Detected
	}
	return targetsFor(s.Name, s.Seats)
}

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
//
// An adopted module can have several, so the game's own name breaks the
// tie before the kind does — a repository whose entry is named after the
// game is naming the thing a developer runs.
func (s *Spec) DevTarget() string {
	targets := s.Targets()
	for _, t := range targets {
		if t.Name == s.Name {
			return t.Name
		}
	}
	for _, t := range targets {
		if t.Kind == "client" || t.Kind == "listen" {
			return t.Name
		}
	}
	if len(targets) > 0 {
		return targets[0].Name
	}
	return ""
}

// Framework reports the framework module path for templates.
func (s *Spec) Framework() string { return FrameworkModule }

// DistillEntry reports the distillation entry package for templates.
func (s *Spec) DistillEntry() string { return DistillEntry }

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

	// ebigent.toml is the only one of these an adopted module wants.
	// The rest describe the placeholder game, and .gitignore and
	// README.md belong to whoever started the repository — writing over
	// either would be init deciding something it was not asked about.
	fixed := map[string]string{"ebigent.toml": "ebigent.toml.tmpl"}
	if !spec.Adopt {
		fixed[".gitignore"] = "gitignore.tmpl"
		fixed["README.md"] = "README.md.tmpl"
		fixed["game/game.go"] = "game.go.tmpl"
		fixed["game/bind.go"] = "bind.go.tmpl"
		fixed["game/game_test.go"] = "game_test.go.tmpl"
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

	// The boundary test needs the playable entry in scope: it is the one
	// allowed to link the engine, and the one whose tagged form must not.
	// It names cmd/<game>, so it only exists where that was generated.
	for _, t := range generatedTargets(spec) {
		if t.Kind == "simulation" {
			continue
		}
		body, err := execute("boundary_test.go.tmpl", struct {
			*Spec
			Target Target
		}{spec, t})
		if err != nil {
			return nil, err
		}
		if body, err = gofmtSource("boundary_test.go", body); err != nil {
			return nil, err
		}
		out["boundary_test.go"] = body
	}

	for _, t := range generatedTargets(spec) {
		data := struct {
			*Spec
			Target Target
		}{spec, t}
		// A server directory holds three files: the shared body and one
		// small file per linkage form, selected by the listen tag.
		files := map[string]string{}
		switch {
		case t.Kind == "simulation":
			files["main.go"] = "main_simulation.go.tmpl"
		case t.Tagged():
			// One directory, two linkage forms: the plain build plays
			// and hosts, the tagged one drops the renderer.
			files["main.go"] = "main_game.go.tmpl"
			files["play.go"] = "game_play.go.tmpl"
			files["dedicated.go"] = "game_dedicated.go.tmpl"
		default:
			files["main.go"] = "main_game.go.tmpl"
			files["play.go"] = "game_play.go.tmpl"
		}
		for base, tmpl := range files {
			body, err := execute(tmpl, data)
			if err != nil {
				return nil, err
			}
			name := path.Join("cmd", t.Dir, base)
			if body, err = gofmtSource(name, body); err != nil {
				return nil, err
			}
			out[name] = body
		}
	}

	// decision:ai-pipeline-always-scaffolded: the corpus root, the chip
	// library, the distillation entry, and the analysis skill exist
	// before there is any data, because recording has to be in place
	// first.
	lib, err := execute("chips.json.tmpl", spec)
	if err != nil {
		return nil, err
	}
	out["behavior/chips.json"] = lib
	out["corpus/.gitkeep"] = []byte{}

	distill, err := execute("main_distill.go.tmpl", spec)
	if err != nil {
		return nil, err
	}
	if distill, err = gofmtSource(DistillEntry+"/main.go", distill); err != nil {
		return nil, err
	}
	out[strings.TrimPrefix(DistillEntry, "./")+"/main.go"] = distill

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

// generatedTargets is the entry points init writes source for, which is
// none of them in an adopted module: its entries are the ones already
// there, and init only learned their names to declare them.
func generatedTargets(spec *Spec) []Target {
	if spec.Adopt {
		return nil
	}
	return spec.Targets()
}

func execute(name string, data any) ([]byte, error) {
	t, err := template.New(name).Funcs(template.FuncMap{
		"addOne": func(i int) int { return i + 1 },
		// half splits a seat count into two teams for the commented
		// example, so the sample division always adds up.
		"half": func(i int) int { return i / 2 },
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

// DirectDeps are the modules the generated sources import by name,
// beyond the framework itself.
func (s *Spec) DirectDeps() []string {
	deps := []string{"github.com/shibukawa/fixmath"}
	for _, t := range s.Targets() {
		// The playable entry renders whether or not it also hosts.
		if t.Kind == "client" || t.Kind == "listen" {
			return append(deps, "github.com/hajimehoshi/ebiten/v2")
		}
	}
	return deps
}

// InitModule builds the generated project's go.mod with the go tool
// rather than writing one.
//
// A written go.mod freezes whatever the template said — a go directive
// from whenever it was authored, and a version for every dependency. go
// mod init writes the directive the installed toolchain actually wants,
// and go get resolves what is current at the moment the project is made,
// which is the version a new project should start on.
//
// The exception is a local framework checkout: there, the point is to
// build against that checkout, so its own requirements are what the
// project should inherit. tidy reads them straight out of the replaced
// directory, and asking a proxy for anything newer would only introduce a
// version skew the developer did not ask for.
func InitModule(dir string, spec *Spec, env []string) error {
	if err := goRun(dir, env, "mod", "init", spec.Module); err != nil {
		return err
	}
	return Require(dir, spec, env)
}

// Require adds the framework to a module that already has a go.mod,
// which is everything InitModule does except declaring the module.
//
// It is the same call in both directions on purpose. An adopted project
// needs the framework required, the codec generator recorded as a tool,
// and tidy run over the entry init just wrote — no less than a generated
// one does, and by the same route, so there is one description of what
// depending on ebigent means.
func Require(dir string, spec *Spec, env []string) error {
	if spec.FrameworkPath != "" {
		if err := goRun(dir, env, "mod", "edit",
			"-require="+FrameworkModule+"@v0.0.0",
			"-replace="+FrameworkModule+"="+spec.FrameworkPath); err != nil {
			return err
		}
		if err := addTool(dir, env); err != nil {
			return err
		}
		return Tidy(dir, env)
	}
	for _, mod := range append([]string{FrameworkModule}, spec.DirectDeps()...) {
		if err := goRun(dir, env, "get", mod); err != nil {
			return err
		}
	}
	if err := addTool(dir, env); err != nil {
		return err
	}
	return Tidy(dir, env)
}

// ModulePath reads the module path out of a go.mod, and reports
// whether there was one to read.
//
// Missing is not an error: it is the question init asks to decide which
// of its two jobs it is doing. A go.mod that exists but declares nothing
// is an error, because that module cannot be built either way.
func ModulePath(dir string) (string, bool, error) {
	body, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	for line := range strings.Lines(string(body)) {
		line = strings.TrimSpace(line)
		if line, ok := strings.CutPrefix(line, "module"); ok {
			path := strings.TrimSpace(line)
			if p, err := strconv.Unquote(path); err == nil {
				path = p
			}
			if path == "" {
				break
			}
			return path, true, nil
		}
	}
	return "", false, fmt.Errorf("scaffold: %s declares no module path", filepath.Join(dir, "go.mod"))
}

// DetectTargets reports the entry points an existing module already has,
// so init can declare them instead of inventing directories beside them.
//
// The kind is read off the import graph rather than guessed from the
// name: rule:engine-import-confined-to-client-entry says the engine
// reaches exactly one kind of entry, so an entry that transitively links
// Ebitengine is a playing one and an entry that does not is headless.
// That is the same distinction concept:build-target draws, arrived at
// from the only evidence an adopted repository actually carries.
//
// The distillation entry is excluded. It is a tool init itself writes,
// not an artifact the project ships, and listing it would put it in
// front of `ebigent build`.
func DetectTargets(dir string, env []string) ([]Target, error) {
	// -e keeps a package that does not compile in the listing: an
	// adopted module is often mid-edit, and refusing to configure it
	// until it builds would be the wrong order.
	out, err := goOutput(dir, env, "list", "-e", "-f", "{{.Name}}\t{{.ImportPath}}\t{{.Dir}}\t{{join .Deps \" \"}}", "./...")
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var targets []Target
	for line := range strings.Lines(out) {
		fields := strings.Split(strings.TrimRight(line, "\n"), "\t")
		if len(fields) < 3 || fields[0] != "main" {
			continue
		}
		rel, err := filepath.Rel(root, fields[2])
		if err != nil {
			continue
		}
		// A main package at the module root is "." and not "./.", which
		// go build accepts and a person reading the configuration does
		// not. The name comes from the import path rather than the
		// directory for the same reason: at the root the directory is a
		// dot and the import path is the module.
		rel = filepath.ToSlash(rel)
		entry := "./" + rel
		if rel == "." {
			entry = "."
		}
		if entry == DistillEntry {
			continue
		}
		kind := "simulation"
		if len(fields) > 3 && slices.Contains(strings.Fields(fields[3]), EngineModule) {
			kind = "client"
		}
		targets = append(targets, Target{Name: path.Base(fields[1]), Dir: rel, Kind: kind, Path: entry})
	}
	slices.SortFunc(targets, func(a, b Target) int { return strings.Compare(a.Entry(), b.Entry()) })
	return targets, nil
}

// EngineModule is the renderer whose presence in an import graph makes
// an entry a playing one.
const EngineModule = "github.com/hajimehoshi/ebiten/v2"

// CodecGenerator is the tool `ebigent generate` drives to write codecs.
const CodecGenerator = "github.com/shibukawa/tinybind-go/cmd/tinybind-gen"

// addTool records the codec generator as a tool dependency, before tidy
// runs so tidy resolves what it needs.
//
// A tool directive is what makes the generator's own dependencies
// reachable. tidy pins what the game imports, and the generator is not
// one of those — the game imports the runtime, never the thing that
// writes against it — so without this the first generate fails on a
// go.sum the project had no reason to have.
//
// It edits rather than gets: the version is already settled by whatever
// required tinybind, and asking a proxy to resolve it again would make
// project creation need a network it otherwise does not.
func addTool(dir string, env []string) error {
	return goRun(dir, env, "mod", "edit", "-tool="+CodecGenerator)
}

// Tidy runs go mod tidy, resolving whatever the sources import and
// writing go.sum.
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
	_, err := goOutput(dir, env, args...)
	return err
}

func goOutput(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("scaffold: go %s: %w\n%s", strings.Join(args, " "), err, stderr.Bytes())
	}
	return string(out), nil
}

// Realtime is the concept:realtime-intensity a project starts at.
//
// A starting value, not a constraint. The wizard does not ask it outright
// because the play style already implies it: duo exists as a style
// because the tight input-to-pixel loop is the point, and everything else
// begins where a frame of delay is invisible. A puzzle game edits this to
// turn_based and loses nothing by having started elsewhere.
func (s *Spec) Realtime() string {
	if s.Style == "duo" {
		return "twitch"
	}
	return "paced"
}

// View is the concept:view-arrangement, which is the shared-screen answer
// under the name the configuration uses.
func (s *Spec) View() string {
	if s.SharedScreen {
		return "shared"
	}
	return "per_agent"
}

// ProtocolSync is the concept:synchronization-mode the [protocol] table
// declares. A solo game has no link and therefore no mode to choose, so
// it takes the authoritative value rather than leaving the key empty:
// there is nothing to synchronize either way, and an empty enum is a
// validation error rather than a statement.
func (s *Spec) ProtocolSync() string {
	if s.SyncMode == "" {
		return "server_authoritative"
	}
	return s.SyncMode
}

// Devices are the input devices the generated game accepts. The
// placeholder reads keys and nothing else; a game that grows a cursor
// adds mouse here and writes the api:input-adapter to match.
func (s *Spec) Devices() []string { return []string{"keyboard"} }
