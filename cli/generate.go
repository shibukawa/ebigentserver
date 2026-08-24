package cli

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/shibukawa/ebigentserver/codegen"
	"github.com/shibukawa/ebigentserver/config/buildconf"
)

// GeneratedDir is where ebigent generate writes, relative to the project
// root. One directory rather than files beside the hand-written source,
// so a stale artifact is visible and `rm -rf` is a safe repair.
const GeneratedDir = "internal/ebigentgen"

// generatedFile is the protocol constants file inside GeneratedDir.
const generatedFile = "protocol_gen.go"

// runGenerate emits the protocol level of concept:configuration-scope as
// Go constants (requirement:config-codegen).
//
// Nothing in [protocol] can differ between two launches of one artifact,
// so nothing in it is worth a startup lookup: the build settles it, and a
// constant is what "settled" looks like from the code that reads it.
//
// It is idempotent — same table, same bytes — so a no-op run leaves the
// tree unchanged and a diff shows only what the table actually changed.
func runGenerate(c *context) error {
	if err := c.requireProject(); err != nil {
		return err
	}
	src, err := generateProtocol(c.build)
	if err != nil {
		return err
	}
	dir := filepath.Join(c.res.ProjectRoot, filepath.FromSlash(GeneratedDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if err := writeIfChanged(c, filepath.Join(dir, generatedFile), src); err != nil {
		return err
	}
	return generateCodecs(c)
}

// writeIfChanged writes only when the bytes differ, and says which it did.
//
// A no-op run has to leave the mtime alone: flow:dev-rebuild-loop watches
// the tree, so a generator that rewrote identical bytes would make every
// rebuild trigger the next one.
func writeIfChanged(c *context, path string, src []byte) error {
	rel, err := filepath.Rel(c.res.ProjectRoot, path)
	if err != nil {
		rel = path
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, src) {
		fmt.Fprintf(c.stdout, "%s (unchanged)\n", rel)
		return nil
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	fmt.Fprintf(c.stdout, "%s\n", rel)
	return nil
}

// tinybindGen is the codec generator ebigent drives, named as init
// recorded it. A game asks for a codec by calling an entry point; running
// the tool that answers is ebigent's job, not something a game repeats in
// a //go:generate comment beside every message type.
const tinybindGen = "tinybind-gen"

// generateCodecs emits the two halves of concept:state-synchronization
// for every package that asks: the whole-value codecs, which tinybind
// writes, and the delta, which since v0.5.23 no library writes
// (requirement:cborbind-migration).
//
// Both halves run from one command because they answer one question. A
// game that had to remember `go generate` for the codec and
// `ebigent generate` for the delta would be keeping the framework's build
// order in its head, and the first symptom of forgetting is a delta
// computed against a codec that moved.
//
// Which packages those are is not configured. A package that imports the
// runtime is asking, and a type asked for the map shape is a world state
// — asking is the declaration, the same idiom tinybind adopted — so there
// is no list to keep in step with the source.
func generateCodecs(c *context) error {
	dirs, err := worldPackages(c.res.ProjectRoot)
	if err != nil {
		return err
	}
	asks, err := codegen.Stages(c.res.ProjectRoot)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if asks[dir], err = c.askAndGenerate(dir, asks[dir]); err != nil {
			return err
		}
		names, err := codegen.Discover(dir)
		if err != nil {
			return err
		}
		names = codegen.SortedNames(append(names, codegen.AskedWorlds(asks[dir])...))
		if len(names) == 0 {
			continue
		}
		pkg, structs, err := codegen.Read(dir, names)
		if err != nil {
			return err
		}
		src, err := codegen.Emit(pkg, structs)
		if err != nil {
			return err
		}
		if err := writeIfChanged(c, filepath.Join(dir, "delta_gen.go"), src); err != nil {
			return err
		}
		if err := writeIfChanged(c, filepath.Join(dir, "schema_gen.go"), codegen.EmitVersion(pkg, structs)); err != nil {
			return err
		}
	}
	return nil
}

// askAndGenerate writes one package's asks, runs the codec generator over
// them, and returns the ones that came back whole.
//
// An ask the generator cannot answer is withdrawn and reported rather
// than kept: the file is rewritten without it and the generator runs
// again, so what lands is a codec for everything that works and nothing
// for what does not. A half-written codec that compiles is the outcome
// worth avoiding, because it loses members quietly on every send.
func (c *context) askAndGenerate(dir string, asks []codegen.Ask) ([]codegen.Ask, error) {
	// Asking is how the answer is found out, so a withdrawn ask means
	// writing the file twice. Remembering what was there lets the second
	// write undo the first completely, mtime included, or
	// flow:dev-rebuild-loop would see a change every pass and rebuild
	// because it just rebuilt.
	before := c.snapshot(dir, askFile, codecFile)
	if len(asks) > 0 {
		if err := c.writeAsks(dir, asks); err != nil {
			return nil, err
		}
	}
	if err := runCodecGenerator(c, dir, len(asks) > 0); err != nil {
		return nil, err
	}
	if len(asks) == 0 {
		return nil, nil
	}
	// A generated codec imports the CBOR runtime, which nothing the game
	// wrote imports, so the module graph is one dependency short until
	// the first codec lands. Settling it here rather than at init keeps
	// the version the one the toolchain resolves when it is first
	// needed, and means a project that never declares a stage never
	// requires it at all.
	if err := c.tidyOnce(); err != nil {
		return nil, err
	}
	kept, problems := codegen.CheckAsks(dir, asks)
	if len(problems) == 0 {
		return kept, nil
	}
	rel, err := filepath.Rel(c.res.ProjectRoot, dir)
	if err != nil {
		rel = dir
	}
	for _, p := range problems {
		fmt.Fprintf(c.stderr, "%s: %v\n", rel, p)
	}
	if err := c.writeAsks(dir, kept); err != nil {
		return nil, err
	}
	if err := runCodecGenerator(c, dir, len(kept) > 0); err != nil {
		return nil, err
	}
	restore(before)
	return kept, nil
}

// codecFile is what the codec generator writes.
const codecFile = "tinybind_gen.go"

// stamp is one file as it stood before generation touched it.
type stamp struct {
	path string
	body []byte
	mod  time.Time
}

// snapshot records files whose content may end up where it started.
func (c *context) snapshot(dir string, names ...string) []stamp {
	out := make([]stamp, 0, len(names))
	for _, n := range names {
		path := filepath.Join(dir, n)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, stamp{path: path, body: body, mod: info.ModTime()})
	}
	return out
}

// restore puts back the modification time of every file that ended the
// run holding exactly what it held at the start.
func restore(before []stamp) {
	for _, s := range before {
		if body, err := os.ReadFile(s.path); err == nil && bytes.Equal(body, s.body) {
			_ = os.Chtimes(s.path, s.mod, s.mod)
		}
	}
}

// tidyOnce settles the module graph, at most once per generate.
func (c *context) tidyOnce() error {
	if c.tidied {
		return nil
	}
	c.tidied = true
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = c.res.ProjectRoot
	cmd.Env = os.Environ()
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("generate: settling the module graph: %w\n%s", err, combined)
	}
	return nil
}

// writeAsks puts one package's asks on disk, or removes the file when
// there are none left to make.
func (c *context) writeAsks(dir string, asks []codegen.Ask) error {
	path := filepath.Join(dir, askFile)
	if len(asks) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("generate: %w", err)
		}
		return nil
	}
	pkg, err := codegen.PackageName(dir)
	if err != nil {
		return err
	}
	return writeIfChanged(c, path, codegen.EmitAsks(pkg, asks))
}

// askFile is where the generated codec asks go. It sits beside the types
// it names rather than in GeneratedDir, because the codec generator reads
// one package at a time and the ask has to be in the package.
const askFile = "wire_gen.go"

// runCodecGenerator runs the codec generator in one package, and only in
// a package that asked for one.
//
// The check is what keeps the walk cheap: a project is mostly packages
// with no wire types at all, and starting a toolchain process in each of
// them to be told there is nothing to do would make generate cost more
// than the build it precedes.
func runCodecGenerator(c *context, dir string, asked bool) error {
	if !asked {
		needs, err := codegen.NeedsCodecs(dir)
		if err != nil || !needs {
			return err
		}
	}
	cmd := exec.Command("go", "tool", tinybindGen, "generate", "-openapi=false")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if combined, err := cmd.CombinedOutput(); err != nil {
		rel, relErr := filepath.Rel(c.res.ProjectRoot, dir)
		if relErr != nil {
			rel = dir
		}
		return fmt.Errorf("generate: codecs for %s: %w\n%s", rel, err, combined)
	}
	return nil
}

// worldPackages are the directories under root that hold Go source, in a
// stable order. Discover then decides which of them declare anything.
//
// Walking beats configuring here: a game that grows a second stage adds a
// package and nothing else, which is the point of
// decision:codec-set-per-stage.
func worldPackages(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "bin") {
			return fs.SkipDir
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				out = append(out, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	sort.Strings(out)
	return out, nil
}

// generateProtocol renders the constants for one validated configuration.
// It is separate from the writing so a test can read the source without a
// project on disk.
func generateProtocol(b *buildconf.Config) ([]byte, error) {
	pkg := b.GamePackage()
	seats := make([]seatFact, 0, b.Protocol.Seats.Count)
	for i := range b.Protocol.Seats.Count {
		team, _ := b.TeamOf(i)
		seats = append(seats, seatFact{
			Slot:     i + 1,
			Team:     team,
			Occupant: b.SeatOccupant(i),
		})
	}
	axes := make([]axisFact, 0, len(b.Protocol.Condition))
	for _, a := range b.Protocol.Condition {
		axes = append(axes, axisFact{Name: a.Name, Band: a.Match == "band"})
	}
	var buf bytes.Buffer
	err := protocolTemplate.Execute(&buf, protocolFacts{
		Axes:     axes,
		Package:  pkg,
		Title:    b.GameTitle(),
		Shape:    b.Protocol.Shape,
		Realtime: b.Protocol.Realtime,
		View:     b.Protocol.View,
		Sync:     b.Protocol.Sync,
		Devices:  b.Protocol.Devices,
		Seats:    seats,
		Fill:     b.Protocol.Seats.Fill,
		Teamed:   len(b.Protocol.Team) > 0,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("generate: emitted invalid Go: %w", err)
	}
	return src, nil
}

// protocolFacts is what the template renders.
type protocolFacts struct {
	Package  string
	Title    string
	Shape    string
	Realtime string
	View     string
	Sync     string
	Devices  []string
	Seats    []seatFact
	Fill     string
	Teamed   bool
	Axes     []axisFact
}

// axisFact is one condition axis, flattened to what matchmaking compares
// on rather than how it was declared.
type axisFact struct {
	Name string
	Band bool
}

// seatFact is one declared seat, resolved through the team division so
// the generated table needs no lookup of its own.
type seatFact struct {
	Slot     int
	Team     string
	Occupant string
}

func quote(s string) string { return strconv.Quote(s) }

var protocolTemplate = template.Must(template.New("protocol").Funcs(template.FuncMap{
	"quote": quote,
}).Parse(`// Code generated by ebigent generate; DO NOT EDIT.
//
// These are the protocol level of concept:configuration-scope: the game's
// own contract, settled in ebigent.toml before this artifact existed. They
// are constants rather than settings because nothing here can differ
// between two launches of one build — reading them at startup would only
// pretend the answer were still open (rule:config-tier-placement).
//
// Edit [protocol] in ebigent.toml and run ebigent generate.

package ebigentgen

// Package is the import path identifying this game, subpath included —
// half of the pair two peers compare before they play
// (decision:module-path-is-game-identity).
const Package = {{quote .Package}}

// Title is what a browse list and a window caption show. It identifies
// nothing, so it may be reworded or localized freely.
const Title = {{quote .Title}}

// Shape is concept:participant-shape, Realtime is
// concept:realtime-intensity, and View is concept:view-arrangement.
const (
	Shape    = {{quote .Shape}}
	Realtime = {{quote .Realtime}}
	View     = {{quote .View}}
)

// Sync is concept:synchronization-mode. It is settled here rather than per
// launch because a mode is a property of the game, and offering it at
// startup is the mistake rule:build-tag-only-for-linkage names in its own
// domain.
const Sync = {{quote .Sync}}

// Devices are the input devices this build accepts. A game cannot accept
// one it never wrote an api:input-adapter for.
var Devices = []string{ {{- range $i, $d := .Devices}}{{if $i}}, {{end}}{{quote $d}}{{end -}} }

// SeatCount is how many concept:player-slot entries the rules declare.
const SeatCount = {{len .Seats}}

// FillUnclaimedSeats reports what a match does with a seat nobody took:
// true completes the roster with a bot so play can start, false starts
// short.
const FillUnclaimedSeats = {{eq .Fill "bots"}}

// Teamed reports whether the game divides its seats at all.
const Teamed = {{.Teamed}}

// Seat is one declared seat, with the team division already resolved.
type Seat struct {
	// Slot is the concept:player-slot id, counting from one.
	Slot uint16
	// Team is the team it belongs to, empty when the game has none.
	Team string
	// Occupant bounds who may take it: any, human, or bot.
	Occupant string
}

// Seats is the declared seat composition. api:roster fills these and
// never invents one.
var Seats = []Seat{
{{- range .Seats}}
	{Slot: {{.Slot}}, Team: {{quote .Team}}, Occupant: {{quote .Occupant}}},
{{- end}}
}

// Axis is one term matchmaking may filter on. Band marks the asymmetric
// comparison: the room states a range and a joiner brings their own
// value, since a rank is something a player has rather than something
// they pick.
type Axis struct {
	Name string
	Band bool
}

// Axes is the condition set this game declares
// (requirement:conditional-matchmaking). Both ends compare the same axes
// because both were built from this table, which is what lets a refusal
// name the term it failed.
var Axes = []Axis{
{{- range .Axes}}
	{Name: {{quote .Name}}, Band: {{.Band}}},
{{- end}}
}
`))
