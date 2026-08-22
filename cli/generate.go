package cli

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

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
	return generateDeltas(c)
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

// generateDeltas emits the delta half of concept:state-synchronization for
// every package declaring a world state, which since v0.5.23 no library
// does (requirement:cborbind-migration).
//
// Which packages those are is not configured. A type asked for the map
// shape is a world state, and asking is the declaration — the same idiom
// tinybind adopted — so there is no list to keep in step with the source.
func generateDeltas(c *context) error {
	dirs, err := worldPackages(c.res.ProjectRoot)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		names, err := codegen.Discover(dir)
		if err != nil {
			return err
		}
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
	var buf bytes.Buffer
	err := protocolTemplate.Execute(&buf, protocolFacts{
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
`))
