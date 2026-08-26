package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/shibukawa/ebigentserver/analysis"
	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/codegen"
	"github.com/shibukawa/ebigentserver/config/buildconf"
	"github.com/shibukawa/ebigentserver/config/confload"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/scaffold"
	"github.com/shibukawa/tinybind-go/configbind"
)

// runConfig renders the configuration surface. scaffold and env write the
// declared shape; show reports the effective values with the layer that
// set each, which is the startup dump of requirement:layered-configuration
// made available on demand.
func runConfig(c *context, opts *ConfigOptions) error {
	switch opts.Action {
	case "scaffold":
		return configbind.WriteScaffoldTOML(c.stdout)
	case "env":
		return configbind.WriteScaffoldEnv(c.stdout)
	case "show", "":
		if c.res.ProjectRoot != "" {
			fmt.Fprintf(c.stdout, "# project: %s\n", c.res.ProjectRoot)
		} else {
			fmt.Fprintln(c.stdout, "# no project file found; showing defaults, environment, and options")
		}
		return confload.WriteProvenance(c.stdout, c.res)
	default:
		return fmt.Errorf("unknown config action %q; use scaffold, env, or show", opts.Action)
	}
}

// runBuild compiles one declared concept:build-target by building its
// entry point. The target's GOOS and GOARCH travel with it, which is how
// a wasm client and a dedicated server come out of the same command.
func runBuild(c *context, opts *BuildOptions) error {
	if err := c.requireProject(); err != nil {
		return err
	}
	// A target compiled against stale constants is the failure generation
	// exists to prevent, so it is not a step anybody has to remember
	// (requirement:config-codegen).
	if err := runGenerate(c); err != nil {
		return err
	}
	name := opts.Target
	if name == "" {
		t, ok := c.build.DevTarget()
		if !ok {
			return fmt.Errorf("no target given and dev.target does not name one")
		}
		name = t.Name
	}
	var target = struct {
		found bool
		kind  string
		entry string
		goos  string
		arch  string
		tags  []string
	}{}
	for _, t := range c.build.Build.Target {
		if t.Name == name {
			target = struct {
				found bool
				kind  string
				entry string
				goos  string
				arch  string
				tags  []string
			}{true, t.Kind, t.Entry, t.GOOS, t.GOARCH, t.Tags}
		}
	}
	if !target.found {
		return fmt.Errorf("no target named %q in %s", name, c.res.Load.ConfigPath)
	}

	out := opts.Output
	if out == "" {
		out = filepath.Join(c.res.ProjectRoot, "bin", name)
		if target.goos == "js" || target.goos == "wasip1" {
			out += ".wasm"
		} else if target.goos == "windows" || (target.goos == "" && runtime.GOOS == "windows") {
			out += ".exe"
		}
	}

	args := []string{"build", "-o", out}
	if len(target.tags) > 0 {
		args = append(args, "-tags", strings.Join(target.tags, ","))
	}
	args = append(args, target.entry)

	cmd := exec.Command("go", args...)
	cmd.Dir = c.res.ProjectRoot
	cmd.Env = os.Environ()
	if target.goos != "" {
		cmd.Env = append(cmd.Env, "GOOS="+target.goos)
	}
	if target.arch != "" {
		cmd.Env = append(cmd.Env, "GOARCH="+target.arch)
	}
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build %s: %w\n%s", name, err, combined)
	}
	fmt.Fprintf(c.stdout, "%s (%s) -> %s\n", name, target.kind, out)
	return nil
}

// runAnalyze is the corpus-report tool as a verb: metric:balance-signals
// aggregates over a recorded corpus, plus an optional DuckDB script for
// the deeper cuts.
//
// It reads recorded files after the fact and never links into a game
// process (rule:analysis-tooling-outside-game-process), which is why it
// can live in the same binary as everything else without dragging a
// session into the toolchain.
func runAnalyze(c *context, opts *AnalyzeOptions) error {
	corpus := opts.Corpus
	if corpus == "" {
		if err := c.requireProject(); err != nil {
			return err
		}
		corpus = c.path(c.build.Behavior.Corpus)
	}
	loaded, err := analysis.LoadCorpus(corpus)
	if err != nil {
		return err
	}
	analysis.Compute(loaded).WriteText(c.stdout)

	if opts.SQL == "" {
		return nil
	}
	f, err := os.Create(opts.SQL)
	if err != nil {
		return err
	}
	werr := analysis.WriteDuckDBSQL(f, corpus)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	fmt.Fprintf(c.stdout, "\nduckdb script written: %s (run: duckdb -init %s)\n", opts.SQL, opts.SQL)
	return nil
}

// runMerge is the behavior-merge tool as a verb. Proposals are
// re-validated here against the analysis request regardless of any
// validation that already happened upstream, then merged as a diff that
// never silently overwrites an approved or rejected chip
// (rule:regeneration-preserves-approved-nodes).
func runMerge(c *context, opts *MergeOptions) error {
	library := opts.Library
	if library == "" {
		if err := c.requireProject(); err != nil {
			return err
		}
		library = c.path(c.build.Behavior.Library)
	}

	reqBytes, err := os.ReadFile(opts.Request)
	if err != nil {
		return err
	}
	var req behavior.AnalysisRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	props, err := behavior.LoadProposals(opts.Proposals)
	if err != nil {
		return err
	}
	lib, err := behavior.LoadLibrary(library)
	if os.IsNotExist(err) {
		lib = &behavior.Library{Game: req.Game}
	} else if err != nil {
		return err
	}

	valid, issues := behavior.ValidateProposals(req, props)
	for _, is := range issues {
		fmt.Fprintf(c.stdout, "  [%s] %s: %s\n", is.Kind, is.Candidate, is.Detail)
	}

	diff := behavior.Merge(lib, valid)
	counts := map[behavior.DiffClass]int{}
	for _, d := range diff {
		counts[d.Class]++
		fmt.Fprintf(c.stdout, "  %-24s %s -> %s (coverage %d, counter %d)\n",
			d.Class, d.Candidate.Condition, d.Candidate.Action,
			d.Candidate.Coverage, d.Candidate.Counterexamples)
	}
	if err := lib.Save(library); err != nil {
		return err
	}
	if opts.Diff != "" {
		body, err := json.MarshalIndent(diff, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(opts.Diff, append(body, '\n'), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(c.stdout, "merged into %s: %d new, %d unchanged, %d metric changes, %d rejected-again, %d conflicts; %d validation issues\n",
		library, counts[behavior.DiffNew], counts[behavior.DiffUnchanged], counts[behavior.DiffMetrics],
		counts[behavior.DiffRejectedAgain], counts[behavior.DiffConflict], len(issues))
	if len(props.Predicates) > 0 {
		fmt.Fprintf(c.stdout, "%d new predicate proposals await a developer (implement and review them into the game's predicate package):\n", len(props.Predicates))
		for _, pr := range props.Predicates {
			fmt.Fprintf(c.stdout, "  %s - %s\n", pr.Name, pr.Doc)
		}
	}
	// Nothing here approves anything: rule:generated-behavior-requires-approval
	// makes the editor the only gate, and this verb only prepares the diff.
	return nil
}

// runDoctor reports what would stop the other verbs from working. It
// answers "why did that fail" before it is asked, which is the whole
// point of having the verb.
func runDoctor(c *context) error {
	ok := true
	line := func(good bool, format string, args ...any) {
		mark := "ok  "
		if !good {
			mark = "FAIL"
			ok = false
		}
		fmt.Fprintf(c.stdout, "%s %s\n", mark, fmt.Sprintf(format, args...))
	}

	goBin, err := exec.LookPath("go")
	line(err == nil, "go toolchain: %s", either(goBin, "not on PATH"))
	installed := ""
	if err == nil {
		if out, verr := exec.Command("go", "version").Output(); verr == nil {
			installed = goVersion(string(out))
			line(true, "go version: %s", strings.TrimSpace(string(out)))
		}
	}

	if c.res.ProjectRoot == "" {
		line(false, "project: no %s found in this directory or any parent", "ebigent.toml")
		fmt.Fprintln(c.stdout, "\nrun `ebigent init` to make a project here")
		return nil
	}
	line(true, "project: %s", c.res.ProjectRoot)
	if c.build.Project.Module != "" {
		line(true, "module: %s", c.build.Project.Module)
	}
	// A pinned toolchain is only worth pinning if something notices when
	// the host does not meet it.
	if pin := c.build.Project.GoToolchain; pin != "" && installed != "" {
		line(!olderThan(installed, pin), "go toolchain pin: project wants %s, host has %s", pin, installed)
	}
	line(c.res.Load.ConfigPath != "", "config file: %s", either(c.res.Load.ConfigPath, "none read"))

	if err := c.build.Validate(); err != nil {
		line(false, "project configuration:\n%v", err)
	} else {
		line(true, "project configuration: %d target(s) declared", len(c.build.Build.Target))
		line(true, "protocol: %s, %s, %d seat(s), sync %s",
			c.build.GamePackage(), c.build.Protocol.Shape, c.build.Protocol.Seats.Count, c.build.Protocol.Sync)
	}
	if err := c.run.Validate(); err != nil {
		line(false, "run configuration:\n%v", err)
	} else {
		line(true, "run configuration: topology %s, %d/%d tick/send",
			c.run.Topology, c.run.Tuning.TickRate, c.run.Tuning.SendRate)
	}

	corpus := c.path(c.build.Behavior.Corpus)
	_, statErr := os.Stat(corpus)
	line(statErr == nil, "corpus root: %s", corpus)
	if _, statErr := os.Stat(c.path(c.build.Behavior.Library)); statErr != nil {
		line(false, "chip library: %s is missing", c.path(c.build.Behavior.Library))
	} else {
		line(true, "chip library: %s", c.path(c.build.Behavior.Library))
	}

	if !ok {
		return fmt.Errorf("doctor found problems")
	}
	return nil
}

// goVersion pulls "1.26.7" out of "go version go1.26.7 darwin/arm64".
func goVersion(out string) string {
	for _, f := range strings.Fields(out) {
		if v, ok := strings.CutPrefix(f, "go1."); ok {
			return "1." + v
		}
	}
	return ""
}

// olderThan compares dotted numeric versions, shorter meaning unset
// trailing components. A non-numeric component makes the comparison
// unusable, and an unusable comparison must not fail a doctor run.
func olderThan(have, want string) bool {
	h, w := strings.Split(have, "."), strings.Split(want, ".")
	for i := range max(len(h), len(w)) {
		hv, wv := 0, 0
		if i < len(h) {
			n, err := strconv.Atoi(h[i])
			if err != nil {
				return false
			}
			hv = n
		}
		if i < len(w) {
			n, err := strconv.Atoi(w[i])
			if err != nil {
				return false
			}
			wv = n
		}
		if hv != wv {
			return hv < wv
		}
	}
	return false
}

func either(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// runSimulate fills a corpus by playing the game against itself.
//
// It is the first half of an AI development cycle and the half that
// takes the time: run headless until there are episodes, mine them,
// regenerate, run again against what came out. `go test -run
// TestSomething -update` does the same work in a project that has one
// test doing all of it, and it says nothing about which part is which.
//
// The division here is the same one `build` and `distill` already draw.
// The verb owns what the project declared — which entry is the
// simulation, where the corpus goes, how many matches a person asked
// for — and the entry owns what happens inside a match, because who
// plays whom is a fact about the game (concept:continuous-match-loop
// lists four pairings and the framework picks none of them).
//
// The loop itself belongs to the entry rather than to this verb, and
// that is not a detail. A match index carries the seed
// (rule:shared-rng-seed), so a run of four hundred reproduces from one
// number only while one process counts them; four hundred launches
// would also pay four hundred process starts to save nothing.
//
// Settings reach the child through the environment layer of the same
// binding it reads at startup, so nothing here is a convention invented
// for the occasion: `ebigent simulate --matches 400` and
// `RUN_EPISODE_MATCHES=400 ./bin/sim` are the same run.
func runSimulate(c *context, opts *SimulateOptions) error {
	if err := c.requireProject(); err != nil {
		return err
	}
	target, err := simulationTarget(c, opts.Target)
	if err != nil {
		return err
	}
	if opts.Build {
		if err := runBuild(c, &BuildOptions{Target: target.Name}); err != nil {
			return err
		}
	}
	bin := filepath.Join(c.res.ProjectRoot, "bin", target.Name)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("simulate: %s is not built; run without --build=false", filepath.Join("bin", target.Name))
	}

	// The child runs with the project root as its working directory, so
	// the corpus reaches it as the same relative path the configuration
	// declares and its own report reads the way a person typed it.
	corpus := either(opts.Corpus, c.build.Behavior.Corpus)
	env := append(os.Environ(), "RUN_EPISODE_ROOT="+corpus)
	matches := opts.Matches
	if matches == 0 {
		matches = c.run.Episode.Matches
	}
	env = append(env, "RUN_EPISODE_MATCHES="+strconv.Itoa(matches))
	if opts.Seed != 0 {
		env = append(env, "RUN_EPISODE_SEED="+strconv.Itoa(opts.Seed))
	}
	// The assignment is checked here rather than left to the child,
	// because a run that names a seat wrongly should not first play four
	// hundred matches to find out.
	agents := either(opts.Agents, c.run.Episode.Agents)
	if agents != "" {
		if _, err := run.ParseAgents(agents); err != nil {
			return err
		}
		env = append(env, "RUN_EPISODE_AGENTS="+agents)
	}

	cmd := exec.Command(bin)
	cmd.Dir = c.res.ProjectRoot
	cmd.Env = env
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stdout
	seating := ""
	if agents != "" {
		seating = ", seating " + agents
	}
	if matches == 0 {
		fmt.Fprintf(c.stdout, "simulate: %s%s, recording into %s until interrupted\n", target.Name, seating, corpus)
	} else {
		fmt.Fprintf(c.stdout, "simulate: %s, %s%s into %s\n", target.Name, plural(matches, "match", "matches"), seating, corpus)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("simulate %s: %w", target.Name, err)
	}
	fmt.Fprintf(c.stdout, "\nMine what it recorded with:\n\n    ebigent distill\n")
	return nil
}

// plural renders a count with the right noun, since "1 matches" in the
// line a person reads most often is worth three lines of code.
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// simulationTarget picks the entry to run.
//
// The kind decides it rather than the name: concept:build-target already
// separates the headless entry from the playing one, so a project that
// declared its targets has already answered this. Several is a real
// question and gets asked as one.
func simulationTarget(c *context, name string) (buildconf.Target, error) {
	var sims []buildconf.Target
	for _, t := range c.build.Build.Target {
		if name != "" {
			if t.Name == name {
				return t, nil
			}
			continue
		}
		if t.Kind == "simulation" {
			sims = append(sims, t)
		}
	}
	if name != "" {
		return buildconf.Target{}, fmt.Errorf("simulate: no target named %q in %s", name, c.res.Load.ConfigPath)
	}
	switch len(sims) {
	case 0:
		return buildconf.Target{}, errors.New("simulate: no target of kind simulation is declared; a headless entry is what fills a corpus, and `ebigent init` writes one")
	case 1:
		return sims[0], nil
	}
	var names []string
	for _, t := range sims {
		names = append(names, t.Name)
	}
	return buildconf.Target{}, fmt.Errorf("simulate: %d simulation targets are declared; name one: %s", len(sims), strings.Join(names, ", "))
}

// runDistill hands the mining step to the project's own entry point.
//
// It is the `ebigent build` shape rather than the `ebigent analyze` one,
// and the difference is not a preference. Analysis reads files, so the
// toolchain can do it alone. Mining cannot: a data:derived-predicate is
// a Go function over a concept:sight, and a prebuilt binary has no way
// to receive one from a module it was compiled without. So the game
// writes the step and this verb spawns it, the way build spawns
// `go build` instead of compiling anything itself.
//
// What crosses the boundary is two paths and nothing else. The verb
// never names a corpus size or a seed: a recipe living here as well as
// in the entry is a second place for it to be wrong, which is exactly
// how a regeneration loop stops closing.
// runCurate is the curate step of requirement:corpus-curation as a verb:
// corpus in, curated corpus out, the report on stdout and beside the
// output as report.json. Mining follows with
// `ebigent distill --corpus <out>/train`; the holdout side exists for
// the entry's evaluation, never for mining.
func runCurate(c *context, opts *CurateOptions) error {
	if opts.Holdout < 0 || opts.Holdout > 100 {
		return fmt.Errorf("curate: --holdout %d is a percent; give 0 to 100", opts.Holdout)
	}
	corpus := opts.Corpus
	if corpus == "" {
		if err := c.requireProject(); err != nil {
			return err
		}
		corpus = c.path(c.build.Behavior.Corpus)
	}
	out := opts.Out
	if out == "" {
		out = strings.TrimRight(corpus, "/\\") + "-curated"
	}
	copts := behavior.CurateOptions{
		Filter:  behavior.RowFilter{AgentKind: opts.AgentKind, Result: opts.Result},
		Cap:     opts.Cap,
		Holdout: float64(opts.Holdout) / 100,
		Seed:    uint64(opts.Seed),
	}
	if opts.Seat > 0 {
		seat := uint16(opts.Seat)
		copts.Filter.Slot = &seat
	}
	rep, err := behavior.Curate(corpus, out, copts)
	if err != nil {
		return err
	}
	// report.json keeps the resolved paths; the terminal gets the ones
	// a person would type.
	rep.Source, rep.Out = displayPath(rep.Source), displayPath(rep.Out)
	rep.WriteText(c.stdout)
	return nil
}

// displayPath shortens an absolute path to the working directory's view
// of it, when that view is not worse.
func displayPath(p string) string {
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(wd, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return rel
}

func runDistill(c *context, opts *DistillOptions) error {
	if err := c.requireProject(); err != nil {
		return err
	}
	entry := either(opts.Entry, c.build.Behavior.Distill)
	if entry == "" {
		return errors.New("distill: no entry point; set behavior.distill or pass --entry")
	}
	// A missing entry is the ordinary state of a project that has not
	// written one yet, so it is worth saying plainly rather than letting
	// the go tool report an empty directory.
	if dir := c.path(filepath.FromSlash(strings.TrimPrefix(entry, "./"))); !hasGoFiles(dir) {
		return fmt.Errorf("distill: %s holds no Go files; run `ebigent init` to write a starting entry, or point behavior.distill at the one this project uses", entry)
	}

	cmd := exec.Command("go", "run", entry)
	cmd.Dir = c.res.ProjectRoot
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stdout
	// The entry reads these rather than taking flags, so an entry that
	// ignores them still runs. A flag it had not declared would fail the
	// spawn instead, which is a worse way to learn that somebody rewrote
	// the file.
	cmd.Env = append(os.Environ(),
		"EBIGENT_CORPUS="+either(opts.Corpus, c.build.Behavior.Corpus),
		"EBIGENT_LIBRARY="+either(opts.Library, c.build.Behavior.Library),
	)

	fmt.Fprintf(c.stdout, "distill: go run %s\n", entry)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("distill %s: %w", entry, err)
	}
	// Nothing here approves anything. The entry mines candidates and
	// regenerates from what a developer has already accepted; the gate
	// itself is ui:behavior-tree-editor
	// (rule:generated-behavior-requires-approval).
	return nil
}

// hasGoFiles reports whether a directory holds anything the go tool
// would compile.
func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}

// AddKinds are the pieces `ebigent add` knows how to write.
var AddKinds = []string{"agent", "stage"}

// runAdd writes the boilerplate a declaration already implies.
//
// The first kind is an agent, and it is the one where the gap is widest.
// An api:agent-interface implementation is four methods over two type
// parameters, and both parameters are decided somewhere else — in the
// rule set assertion, which may name types from a package the agent's
// own file does not import yet. All of that is mechanical and none of it
// is the policy, so a developer copying it out of another game is
// copying the part that has no decisions in it.
//
// What this cannot write is Decide, which is the whole point of the
// file. It is left as a TODO returning no action, which compiles and
// plays: a seat that never acts is a legal seat.
//
// Everything after the kind is asked rather than required. Each answer
// narrows the next — the type comes from the name, the file comes from
// the type — so the questions after the first are usually a matter of
// pressing enter, and an option supplied on the command line is what
// they start at.
func runAdd(c *context, opts *AddOptions) error {
	if err := c.requireProject(); err != nil {
		return err
	}
	if !slices.Contains(AddKinds, opts.Kind) {
		return fmt.Errorf("add: %q is not something to add; the kinds are %s", opts.Kind, strings.Join(AddKinds, ", "))
	}
	w := newWizard(c.stdout, opts.Yes)
	if opts.Kind == "stage" {
		return addStage(c, w, opts)
	}

	// Which game comes first, because it decides the two types every
	// later answer is written against.
	rules, err := askRuleSet(w, c.res.ProjectRoot, opts)
	if err != nil {
		return err
	}
	sight, action := rules.Sight.Qualified(rules.Dir), rules.Action.Qualified(rules.Dir)
	// What it read, before what it will write. A developer who sees the
	// wrong sight here stops now rather than after the file exists.
	where := rules.Package
	if r, err := filepath.Rel(c.res.ProjectRoot, rules.Dir); err == nil {
		where = filepath.ToSlash(r)
	}
	w.note("Writing into %s: sight %s, action %s.", where, sight, action)

	name := w.textValid("Agent name", either(opts.Name, "bot"), agentName)
	spec := &scaffold.AgentSpec{
		Dir:     rules.Dir,
		Package: rules.Package,
		Name:    name,
		Sight:   sight,
		Action:  action,
		Imports: agentImports(rules),
		Root:    c.res.ProjectRoot,
	}
	// The derived answers are shown as answers rather than as rules, so
	// a project that already calls its stand-in Bot edits one field
	// instead of learning how the derivation works.
	spec.Type = w.textValid("Go type name", either(opts.Type, spec.TypeName()), goTypeName)
	spec.File = w.text("File name", either(opts.File, spec.FileName()))

	if _, err := scaffold.WriteAgent(spec); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "\nwrote %s\n", spec.Rel())
	fmt.Fprintf(c.stdout, "\n%s answers from the sight and nothing else. Write Decide; the rest is the shape.\n", spec.TypeName())
	// Seating it is one field, and it is the field a developer does not
	// know to look for: run.Binding is where a game hands its rules to
	// the wrapper, and NewAgent is the only thing on it that decides
	// who fills a seat nobody took.
	fmt.Fprintf(c.stdout, "\nSeat it by naming the factory in run.Binding:\n\n    NewAgent: %s,\n", bindingCall(c.res.ProjectRoot, rules, spec))
	return nil
}

// addStage writes a rule set, which is the declaration everything else
// reads.
//
// It is the mirror of the agent case. An agent is written against types
// that already exist somewhere, so the wizard reads them; a stage is
// where those types are first named, so nothing can be read and the
// questions are about the game rather than about Go.
//
// What it writes compiles and its declaration generates, which is not
// the same as playable: Apply, Advance, and Evaluate are the game and
// none of them can be guessed. Saying so is part of the report.
func addStage(c *context, w *wizard, opts *AddOptions) error {
	name := w.textValid("Stage name", either(opts.Name, "stage"), stageName)
	spec := &scaffold.StageSpec{
		Package: scaffold.StagePackage(name),
		Title:   either(c.build.Protocol.Title, name),
		Root:    c.res.ProjectRoot,
	}
	dir := w.text("Where the rules go", either(opts.Package, spec.Package))
	spec.Dir = c.path(filepath.FromSlash(dir))

	// Both of these are already declared, so the questions start at what
	// the project said about itself rather than at a framework default.
	seats := opts.Seats
	if seats == 0 {
		seats = c.build.Protocol.Seats.Count
	}
	spec.Seats = w.number("How many seats does it declare?", max(seats, 1), 1, 64)

	const (
		moves = "every step, on its own"
		waits = "only when somebody acts"
	)
	def := 0
	if c.build.Protocol.Realtime == "turn_based" {
		def = 1
	}
	switch opts.Realtime {
	case "yes":
		spec.Tick = true
	case "no":
	case "":
		spec.Tick = w.choose("Does the world move?", []string{moves, waits}, def,
			map[string]string{
				moves: "session.TickStageRuleSet — Advance runs every tick whether or not anybody acted",
				waits: "session.StageRuleSet — nothing happens between decisions",
			}) == moves
	default:
		return fmt.Errorf("add stage: realtime is %q; use yes or no", opts.Realtime)
	}

	if _, err := scaffold.WriteStage(spec); err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "\nwrote %s\n", spec.Rel())

	// The codecs are the next step and they are not optional: the world
	// and the action cross a link, and until they are generated nothing
	// can send either.
	fmt.Fprintf(c.stdout, "\nThe declaration is what `ebigent generate` reads. Run it to get the codecs:\n\n    ebigent generate\n")
	fmt.Fprintf(c.stdout, "\nThen fill in World, Action, and Sight, and the three methods that are the game:\n")
	fmt.Fprintf(c.stdout, "    Apply     what one action does\n")
	if spec.Tick {
		fmt.Fprintf(c.stdout, "    Advance   what happens anyway\n")
	}
	fmt.Fprintf(c.stdout, "    Evaluate  when a seat has won, lost, or is still playing\n")
	fmt.Fprintf(c.stdout, "\nUntil Evaluate returns a terminal, a session over these rules never ends.\n")
	return nil
}

// stageName refuses a name that could not be a package.
func stageName(s string) error {
	if !token.IsIdentifier(scaffold.StagePackage(s)) {
		return fmt.Errorf("%q does not give a package name; try bonus or bossfight", s)
	}
	return nil
}

// agentName refuses a policy name the type name is derived from and
// could not be. It reports what was typed rather than what it derived,
// since the derivation is not what the question asked for.
func agentName(s string) error {
	if !token.IsIdentifier(scaffold.AgentTypeName(s)) {
		return fmt.Errorf("%q does not give a Go type name; try tactic or hit_and_run", s)
	}
	return nil
}

// goTypeName refuses anything that is not an identifier.
func goTypeName(s string) error {
	if !token.IsIdentifier(s) {
		return fmt.Errorf("%q is not a Go type name", s)
	}
	return nil
}

// askRuleSet settles which game the agent plays.
//
// One rule set answers itself. Several is a real question, and it is one
// with no defensible default: an agent written against the wrong game
// compiles and means nothing, so an unattended run insists on --package
// rather than picking the first.
func askRuleSet(w *wizard, root string, opts *AddOptions) (codegen.RuleSet, error) {
	sets, err := codegen.RuleSets(root)
	if err != nil {
		return codegen.RuleSet{}, err
	}
	if len(sets) == 0 {
		return codegen.RuleSet{}, errors.New("add: no rule set declared in this project, so there is nothing to write an agent against; `ebigent add stage` writes one")
	}
	labels := make([]string, len(sets))
	help := map[string]string{}
	byLabel := map[string]codegen.RuleSet{}
	for i, s := range sets {
		label, err := filepath.Rel(root, s.Dir)
		if err != nil {
			label = s.Dir
		}
		labels[i] = filepath.ToSlash(label)
		help[labels[i]] = fmt.Sprintf("sight %s, action %s",
			s.Sight.Qualified(s.Dir), s.Action.Qualified(s.Dir))
		byLabel[labels[i]] = s
	}

	if opts.Package != "" {
		want := filepath.ToSlash(filepath.Clean(opts.Package))
		for label, s := range byLabel {
			if label == want || filepath.ToSlash(filepath.Clean(s.Dir)) == want {
				return s, nil
			}
		}
		return codegen.RuleSet{}, fmt.Errorf("add: no rule set is declared in %s; this project declares one in %s",
			opts.Package, strings.Join(labels, ", "))
	}
	if len(sets) == 1 {
		return sets[0], nil
	}
	if w.auto {
		return codegen.RuleSet{}, fmt.Errorf("add: this project declares %d rule sets and there is no default worth taking; name one with --package: %s",
			len(sets), strings.Join(labels, ", "))
	}
	return byLabel[w.choose("Which rules does it play?", labels, 0, help)], nil
}

// agentImports are the packages the generated file needs for the two
// type names it writes. A position declared in the rule set's own
// package is spelled bare and imports nothing.
func agentImports(rules codegen.RuleSet) []string {
	var out []string
	for _, ref := range []codegen.TypeRef{rules.Sight, rules.Action} {
		if ref.Qualified(rules.Dir) != ref.Name {
			out = append(out, ref.Import)
		}
	}
	return out
}

// bindingCall renders the factory as the file holding run.Binding would
// have to write it — bare when that file is in the same package, and
// qualified when it is somewhere else, which is the difference between
// a line that compiles and one that does not.
func bindingCall(root string, rules codegen.RuleSet, spec *scaffold.AgentSpec) string {
	call := "New" + spec.TypeName()
	dir, ok := bindingDir(root)
	if !ok || dir == rules.Dir {
		return call
	}
	return rules.Package + "." + call
}

// bindingDir finds the directory of the file that builds a run.Binding.
// It is a text scan rather than a parse because the answer only decides
// how a printed suggestion is spelled: being wrong costs a package
// qualifier, not a build.
func bindingDir(root string) (string, bool) {
	var found string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || found != "" {
			return err
		}
		if d.IsDir() {
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if bytes.Contains(body, []byte("run.Binding[")) {
			found = filepath.Dir(p)
		}
		return nil
	})
	return found, found != ""
}
