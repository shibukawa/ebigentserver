package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/shibukawa/ebigentserver/config/confload"
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
var AddKinds = []string{"agent"}

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
func runAdd(c *context, opts *AddOptions) error {
	if err := c.requireProject(); err != nil {
		return err
	}
	if !slices.Contains(AddKinds, opts.Kind) {
		return fmt.Errorf("add: %q is not something to add; the kinds are %s", opts.Kind, strings.Join(AddKinds, ", "))
	}

	rules, err := ruleSetFor(c.res.ProjectRoot, opts.Package)
	if err != nil {
		return err
	}
	// The file is written into the rule set's own package, which is
	// where the sight is declared and therefore where the fewest
	// imports are needed. It is also where a reader looks for the
	// rules and their stand-in together.
	spec := &scaffold.AgentSpec{
		Dir:     rules.Dir,
		Package: rules.Package,
		Name:    opts.Name,
		Type:    opts.Type,
		File:    opts.File,
		Sight:   rules.Sight.Qualified(rules.Dir),
		Action:  rules.Action.Qualified(rules.Dir),
		Imports: agentImports(rules),
		Root:    c.res.ProjectRoot,
	}
	if _, err := scaffold.WriteAgent(spec); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "wrote %s\n", spec.Rel())
	fmt.Fprintf(c.stdout, "\n%s answers from the sight and nothing else. Write Decide; the rest is the shape.\n", spec.TypeName())
	// Seating it is one field, and it is the field a developer does not
	// know to look for: run.Binding is where a game hands its rules to
	// the wrapper, and NewAgent is the only thing on it that decides
	// who fills a seat nobody took.
	fmt.Fprintf(c.stdout, "\nSeat it by naming the factory in run.Binding:\n\n    NewAgent: %s,\n", bindingCall(c.res.ProjectRoot, rules, spec))
	return nil
}

// ruleSetFor picks the rule set to write against, insisting on a choice
// when a project declares several rather than guessing which game a
// developer meant.
func ruleSetFor(root, pkg string) (codegen.RuleSet, error) {
	sets, err := codegen.RuleSets(root)
	if err != nil {
		return codegen.RuleSet{}, err
	}
	switch {
	case len(sets) == 0:
		return codegen.RuleSet{}, errors.New("add: no rule set declared in this project; a game states `var _ session.StageRuleSet[World, Action, Sight] = RuleSet{}` beside its rules")
	case pkg == "" && len(sets) == 1:
		return sets[0], nil
	case pkg == "":
		var names []string
		for _, s := range sets {
			r, err := filepath.Rel(root, s.Dir)
			if err != nil {
				r = s.Dir
			}
			names = append(names, filepath.ToSlash(r))
		}
		return codegen.RuleSet{}, fmt.Errorf("add: this project declares %d rule sets; name one with --package: %s",
			len(sets), strings.Join(names, ", "))
	}
	want := filepath.Clean(pkg)
	for _, s := range sets {
		r, err := filepath.Rel(root, s.Dir)
		if err != nil {
			continue
		}
		if filepath.Clean(r) == want || filepath.Clean(s.Dir) == want {
			return s, nil
		}
	}
	return codegen.RuleSet{}, fmt.Errorf("add: no rule set is declared in %s", pkg)
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
