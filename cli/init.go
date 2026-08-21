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
		Dir:    abs,
		Module: opts.Module,
		Name:   opts.Name,
		Style:  opts.Style,
		Seats:  opts.Seats, SyncMode: opts.Sync,
		FrameworkPath: opts.FrameworkPath,
	}

	w := &wizard{in: bufio.NewScanner(os.Stdin), out: c.stdout, auto: opts.Yes}
	if err := ask(w, spec, opts); err != nil {
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
	for _, t := range spec.Targets() {
		fmt.Fprintf(c.stdout, "    cd %s && go run %s\n", dir, t.Entry())
		if t.Tagged() {
			fmt.Fprintf(c.stdout, "    cd %s && go run -tags listen %s   # also plays, links a renderer\n", dir, t.Entry())
		}
	}
	fmt.Fprintln(c.stdout, "\nStart with game/game.go — the placeholder rules the session already runs.")
	return nil
}

// ask fills the unanswered half of the spec, narrowing each question by
// the answers before it: step 2 offers only the synchronization modes
// the chosen topology supports, and step 3 only the targets it can use.
func ask(w *wizard, spec *scaffold.Spec, opts *InitOptions) error {
	if spec.Name == "" {
		spec.Name = defaultName(spec.Dir)
		spec.Name = w.text("Game name", spec.Name)
	}
	if spec.Module == "" {
		spec.Module = w.text("Go module path", "example.com/"+spec.Name)
	}
	// How it is played, then how many, then whether a machine may seat
	// more than one person. Each question is skipped when the answer
	// before it settled the matter.
	if spec.Style == "" {
		const (
			solo  = "one player"
			duo   = "two players"
			multi = "multiplayer"
		)
		switch w.choose(
			"How is it played?",
			[]string{solo, duo, multi}, 0,
			map[string]string{
				solo:  "one seat; no link, nothing to synchronize",
				duo:   "exactly two, and they reach each other in one hop — the case rollback is for",
				multi: "more than two, so every exchange goes through a host either way",
			}) {
		case solo:
			spec.Style = "solo"
		case duo:
			spec.Style = "duo"
		default:
			spec.Style = "multi"
		}
	}
	// solo and duo fix their own seat count; only multi has one to give.
	if spec.Style == "multi" && spec.Seats == 0 {
		spec.Seats = w.number("At most how many players in one session?", 4, 3, 8)
	}
	spec.Seats = scaffold.SeatsForStyle(spec.Style, spec.Seats)

	switch {
	case spec.Style == "solo":
		w.note("One player: nothing to seat beside them, and no link to make.")
	case opts.LocalMultiplayer == "yes":
		spec.LocalMultiplayer = true
	case opts.LocalMultiplayer == "no":
	case opts.LocalMultiplayer != "":
		return fmt.Errorf("local_multiplayer is %q; use yes or no", opts.LocalMultiplayer)
	default:
		spec.LocalMultiplayer = w.yesNo(
			"May several players share one machine, each on their own device?", true)
	}
	if spec.LocalMultiplayer {
		// Whether they read one view or a split one is a rendering
		// choice; either way they are looking at the same machine.
		w.note("One machine means one set of facts: split the view or not, every seat at it may know.")
		w.note("Hiding state between players needs them on separate machines.")
	}

	// Where the traffic goes is not asked. It follows the style, and what
	// remains of it is a run value rather than a code difference
	// (concept:deployment-combination).
	switch spec.Style {
	case "duo":
		w.note("Two players reach each other in one hop, so a direct link is worth having:")
		w.note("WebRTC or a LAN, no server. A dedicated server stays available as a run setting.")
	case "multi":
		w.note("Past two players every exchange is two hops, through a peer host or a server alike.")
		w.note("Starting with a dedicated host because its results can be trusted — but a player")
		w.note("can hold the session just as well, and over WebRTC that needs no backend at all.")
	}
	modes := scaffold.SyncModesFor(spec.Seats)
	if spec.SyncMode == "" && len(modes) > 0 {
		def := 0
		for i, m := range modes {
			if m == scaffold.SyncDefaultFor(spec.Style) {
				def = i
			}
		}
		if len(modes) == 1 {
			spec.SyncMode = modes[0]
			w.note("Synchronization mode: %s", modes[0])
		} else {
			spec.SyncMode = w.choose(
				"Synchronization mode — how sessions stay consistent across the link",
				modes, def,
				map[string]string{
					"server_authoritative": "the host decides; clients predict",
					"delay":                "inputs are buffered so every peer steps together",
					"rollback":             "predict and re-simulate; needs deterministic rules",
					"hybrid":               "authoritative with rollback where it pays",
				})
		}
	}
	// The entry point set follows from the shape rather than being asked
	// about: a shape that needs a client and a server needs both, and
	// offering the choice would only produce projects that cannot play.
	// A simulation target is always among them for the same reason
	// decision:ai-pipeline-always-scaffolded gives.
	var names []string
	for _, t := range spec.Targets() {
		names = append(names, t.Name)
		if t.Tagged() {
			names[len(names)-1] += " (plus a -tags listen form)"
		}
	}
	w.note("Entry points: %s", strings.Join(names, ", "))
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

// yesNo asks a yes or no question.
func (w *wizard) yesNo(prompt string, def bool) bool {
	label := "Y/n"
	if !def {
		label = "y/N"
	}
	if w.auto {
		w.note("%s %v", prompt, def)
		return def
	}
	for {
		fmt.Fprintf(w.out, "%s [%s]: ", prompt, label)
		if !w.in.Scan() {
			return def
		}
		switch strings.ToLower(strings.TrimSpace(w.in.Text())) {
		case "":
			return def
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
		fmt.Fprintln(w.out, "  y or n")
	}
}

// number asks for a count, refusing anything outside the range rather
// than silently clamping it.
func (w *wizard) number(prompt string, def, min, max int) int {
	if w.auto {
		w.note("%s %d", prompt, def)
		return def
	}
	for {
		fmt.Fprintf(w.out, "%s [%d]: ", prompt, def)
		if !w.in.Scan() {
			return def
		}
		answer := strings.TrimSpace(w.in.Text())
		if answer == "" {
			return def
		}
		n, err := strconv.Atoi(answer)
		if err == nil && n >= min && n <= max {
			return n
		}
		fmt.Fprintf(w.out, "  a number between %d and %d\n", min, max)
	}
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
