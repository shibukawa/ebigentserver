package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/shibukawa/ebigentserver/scaffold"
)

// runInit is flow:project-init: three questions, then a project that
// already builds.
//
// Every answer is also a command-line option, so the wizard is skipped
// entirely when the options are supplied — which is what lets init run
// unattended in a test or a script.
func runInit(c *context, opts *InitOptions) error {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	spec := &scaffold.Spec{
		Dir:           abs,
		Module:        opts.Module,
		Name:          opts.Name,
		Topology:      opts.Topology,
		SyncMode:      opts.Sync,
		FrameworkPath: opts.FrameworkPath,
	}
	for _, kind := range opts.Target {
		spec.Targets = append(spec.Targets, scaffold.Target{Name: kind, Kind: kind})
	}

	w := &wizard{in: bufio.NewScanner(os.Stdin), out: c.stdout, auto: opts.Yes}
	if err := ask(w, spec); err != nil {
		return err
	}
	if err := spec.Validate(); err != nil {
		return err
	}

	res, err := scaffold.Generate(spec)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "\nwrote %d files under %s\n", len(res.Files), spec.Dir)
	for _, f := range res.Files {
		fmt.Fprintln(c.stdout, "  ", f)
	}

	if opts.SkipTidy {
		fmt.Fprintln(c.stdout, "\nskipped go mod tidy; run it before building")
		return nil
	}
	fmt.Fprintln(c.stdout, "\nresolving modules...")
	if err := scaffold.Tidy(spec.Dir, nil); err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, "building...")
	if err := scaffold.BuildAll(spec.Dir, nil); err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "\n%s is ready.\n\n", spec.Name)
	for _, t := range spec.Targets {
		fmt.Fprintf(c.stdout, "    cd %s && go run %s\n", dir, t.Entry())
	}
	fmt.Fprintln(c.stdout, "\nStart with game/game.go — the placeholder rules the session already runs.")
	return nil
}

// ask fills the unanswered half of the spec, narrowing each question by
// the answers before it: step 2 offers only the synchronization modes
// the chosen topology supports, and step 3 only the targets it can use.
func ask(w *wizard, spec *scaffold.Spec) error {
	if spec.Name == "" {
		spec.Name = defaultName(spec.Dir)
		spec.Name = w.text("Game name", spec.Name)
	}
	if spec.Module == "" {
		spec.Module = w.text("Go module path", "example.com/"+spec.Name)
	}
	if spec.Topology == "" {
		spec.Topology = w.choose(
			"Execution topology — where the session and its agents run",
			scaffold.Topologies, 0,
			map[string]string{
				"standalone": "one process, no network; the place to start",
				"listen":     "a client that also hosts the session",
				"dedicated":  "a headless server, clients connect to it",
				"p2p":        "peers connect directly, no server in the middle",
			})
	}
	modes := scaffold.SyncModesFor(spec.Topology)
	if len(modes) == 0 {
		return fmt.Errorf("unknown topology %q", spec.Topology)
	}
	if spec.SyncMode == "" {
		if len(modes) == 1 {
			spec.SyncMode = modes[0]
			w.note("Synchronization mode: %s (the only mode %s supports)", modes[0], spec.Topology)
		} else {
			spec.SyncMode = w.choose(
				"Synchronization mode — how sessions stay consistent",
				modes, 0,
				map[string]string{
					"server_authoritative": "the host decides; clients predict",
					"delay":                "inputs are buffered so every peer steps together",
					"rollback":             "predict and re-simulate; needs deterministic rules",
					"hybrid":               "authoritative with rollback where it pays",
				})
		}
	}
	if len(spec.Targets) == 0 {
		kinds := scaffold.TargetsFor(spec.Topology)
		chosen := w.chooseMany(
			"Build targets — one entry point each",
			kinds, []int{0},
			map[string]string{
				"client":     "plays the game; the only entry that may link Ebitengine",
				"listen":     "plays and hosts in the same process",
				"dedicated":  "hosts headless; never links a renderer",
				"simulation": "headless and deterministic; what playtests and training run",
			})
		for _, k := range chosen {
			spec.Targets = append(spec.Targets, scaffold.Target{Name: k, Kind: k})
		}
	}
	// A simulation target costs nothing and is what the AI pipeline runs
	// through, so it is added rather than asked about — the same reasoning
	// as decision:ai-pipeline-always-scaffolded.
	if !slices.ContainsFunc(spec.Targets, func(t scaffold.Target) bool { return t.Kind == "simulation" }) {
		spec.Targets = append(spec.Targets, scaffold.Target{Name: "simulation", Kind: "simulation"})
		w.note("Adding a simulation target: the AI pipeline and automated playtests run through it.")
	}
	return nil
}

// defaultName derives a game name from the target directory.
func defaultName(dir string) string {
	base := filepath.Base(dir)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "game"
	}
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == '-' || r == '_':
			return r
		default:
			return -1
		}
	}, base)
	if name == "" {
		return "game"
	}
	return name
}

// wizard asks the questions. In auto mode it answers every one with the
// default and says so, so an unattended run still reports what it chose.
type wizard struct {
	in   *bufio.Scanner
	out  interface{ Write([]byte) (int, error) }
	auto bool
}

func (w *wizard) note(format string, args ...any) {
	fmt.Fprintf(w.out, format+"\n", args...)
}

func (w *wizard) text(prompt, def string) string {
	if w.auto {
		w.note("%s: %s", prompt, def)
		return def
	}
	fmt.Fprintf(w.out, "%s [%s]: ", prompt, def)
	if !w.in.Scan() {
		return def
	}
	if answer := strings.TrimSpace(w.in.Text()); answer != "" {
		return answer
	}
	return def
}

func (w *wizard) choose(prompt string, options []string, def int, help map[string]string) string {
	if w.auto {
		w.note("%s: %s", prompt, options[def])
		return options[def]
	}
	fmt.Fprintf(w.out, "\n%s\n", prompt)
	for i, o := range options {
		fmt.Fprintf(w.out, "  %d) %-22s %s\n", i+1, o, help[o])
	}
	for {
		fmt.Fprintf(w.out, "choice [%d]: ", def+1)
		if !w.in.Scan() {
			return options[def]
		}
		answer := strings.TrimSpace(w.in.Text())
		if answer == "" {
			return options[def]
		}
		n, err := strconv.Atoi(answer)
		if err == nil && n >= 1 && n <= len(options) {
			return options[n-1]
		}
		fmt.Fprintln(w.out, "  pick a number from the list")
	}
}

func (w *wizard) chooseMany(prompt string, options []string, def []int, help map[string]string) []string {
	pick := func(idx []int) []string {
		out := make([]string, 0, len(idx))
		for _, i := range idx {
			out = append(out, options[i])
		}
		return out
	}
	if w.auto {
		w.note("%s: %s", prompt, strings.Join(pick(def), ", "))
		return pick(def)
	}
	fmt.Fprintf(w.out, "\n%s\n", prompt)
	for i, o := range options {
		fmt.Fprintf(w.out, "  %d) %-22s %s\n", i+1, o, help[o])
	}
	for {
		fmt.Fprintf(w.out, "choices, comma separated [%s]: ", joinInts(def))
		if !w.in.Scan() {
			return pick(def)
		}
		answer := strings.TrimSpace(w.in.Text())
		if answer == "" {
			return pick(def)
		}
		var idx []int
		ok := true
		for _, field := range strings.Split(answer, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(field))
			if err != nil || n < 1 || n > len(options) {
				ok = false
				break
			}
			if !slices.Contains(idx, n-1) {
				idx = append(idx, n-1)
			}
		}
		if ok && len(idx) > 0 {
			return pick(idx)
		}
		fmt.Fprintln(w.out, "  pick numbers from the list, separated by commas")
	}
}

func joinInts(idx []int) string {
	parts := make([]string, len(idx))
	for i, n := range idx {
		parts[i] = strconv.Itoa(n + 1)
	}
	return strings.Join(parts, ",")
}
