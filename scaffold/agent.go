package scaffold

import (
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// AgentSpec is one api:agent-interface implementation to write.
//
// It carries type names rather than types because nothing in the
// toolchain links the game (decision:one-ebigent-binary): the sight and
// the action are read out of the rule set declaration as syntax, and
// they arrive here as the text a file in this package would have to
// write.
type AgentSpec struct {
	// Dir is the package directory the file is written to, and Package
	// its package clause.
	Dir     string
	Package string
	// Name is the id the factory returns — the policy's name, which is
	// what a lobby shows and what labels the seat in a corpus.
	Name string
	// Type is the Go type. Empty derives it from Name.
	Type string
	// File is the base name. Empty derives it from Name.
	File string
	// Sight and Action are the two positions an agent is written
	// against, spelled as this package would spell them.
	Sight, Action string
	// Imports are the paths those spellings need, beyond the session
	// package every agent uses.
	Imports []string
	// Root is the project root. It only shortens paths in messages, so
	// a report names the file the way a person would type it.
	Root string
}

// SessionModule is the package every generated agent implements against.
const SessionModule = FrameworkModule + "/session"

// Validate rejects a spec that could not produce a compiling file.
func (s *AgentSpec) Validate() error {
	var errs []error
	if s.Dir == "" {
		errs = append(errs, errors.New("Dir is required"))
	}
	if !token.IsIdentifier(s.Package) {
		errs = append(errs, fmt.Errorf("package name %q is not an identifier", s.Package))
	}
	if s.Name == "" {
		errs = append(errs, errors.New("Name is required"))
	}
	if !token.IsIdentifier(s.TypeName()) {
		errs = append(errs, fmt.Errorf("%q does not give a Go type name; pass one with --type", s.Name))
	}
	if s.Sight == "" || s.Action == "" {
		errs = append(errs, errors.New("Sight and Action are required"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("scaffold: invalid agent: %w", errors.Join(errs...))
	}
	return nil
}

// TypeName is the Go type this agent declares.
func (s *AgentSpec) TypeName() string {
	if s.Type != "" {
		return s.Type
	}
	return exported(s.Name)
}

// Receiver is the method receiver name, which by Go convention is the
// first letter of the type rather than a word.
func (s *AgentSpec) Receiver() string {
	name := s.TypeName()
	if name == "" {
		return "a"
	}
	return strings.ToLower(name[:1])
}

// FileName is the base name of the file it is written to.
func (s *AgentSpec) FileName() string {
	if s.File != "" {
		return s.File
	}
	return "agent_" + fileSafe(s.Name) + ".go"
}

// Path is where the file lands.
func (s *AgentSpec) Path() string { return filepath.Join(s.Dir, s.FileName()) }

// Rel is Path as a person would type it, when the root is known.
func (s *AgentSpec) Rel() string { return relTo(s.Root, s.Path()) }

// Imports resolves the import block: the session package always, plus
// whatever the sight and action spellings need, deduplicated and sorted
// so the emitted file does not move between runs.
func (s *AgentSpec) importSet() []string {
	out := append([]string{SessionModule}, s.Imports...)
	slices.Sort(out)
	return slices.Compact(out)
}

// WriteAgent writes the implementation and reports the path.
//
// It refuses to overwrite, because an agent is hand-written after this
// point: the file exists to be filled in, so a second run naming the
// same one would throw away the only part worth keeping.
func WriteAgent(spec *AgentSpec) (string, error) {
	if err := spec.Validate(); err != nil {
		return "", err
	}
	path := spec.Path()
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("scaffold: %s already exists; an agent is hand written from here on, so add will not overwrite one", spec.Rel())
	}
	body, err := execute("agent.go.tmpl", struct {
		*AgentSpec
		Package  string
		Type     string
		Receiver string
		Imports  []string
	}{spec, spec.Package, spec.TypeName(), spec.Receiver(), spec.importSet()})
	if err != nil {
		return "", err
	}
	if body, err = gofmtSource(path, body); err != nil {
		return "", err
	}
	if err := os.MkdirAll(spec.Dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// AgentTypeName is the Go type a policy name derives, which a caller
// asking the question needs before the spec exists.
func AgentTypeName(name string) string { return exported(name) }

// exported turns a policy name into a Go type name: "tactic" becomes
// Tactic, "hit_and_run" and "hit-and-run" both become HitAndRun.
func exported(name string) string {
	var b strings.Builder
	upper := true
	for _, r := range name {
		switch {
		case r == '_' || r == '-' || r == ' ':
			upper = true
		case upper:
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// fileSafe turns a policy name into a file name fragment.
func fileSafe(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == '-' || r == ' ':
			return '_'
		default:
			return -1
		}
	}, name)
}

// StageSpec is one concept:stage to write: a rule set, its world, and
// the two types that travel with it.
//
// It is the other half of AgentSpec. An agent is written against a rule
// set that already exists, so its types are read; a stage is where they
// first get named, so nothing can be read and everything is declared.
type StageSpec struct {
	// Dir is the package directory, and Package its package clause.
	Dir     string
	Package string
	// Title names the game in the file's own documentation.
	Title string
	// Seats is how many concept:player-slot entries the rules declare.
	Seats int
	// Tick selects session.TickStageRuleSet over session.StageRuleSet:
	// a world that moves on its own every step rather than only when
	// somebody acts.
	Tick bool
	// Root is the project root, used only to name files in messages.
	Root string
}

// StageFile is the base name of the file a stage is written to.
const StageFile = "rules.go"

// Validate rejects a spec that could not produce a compiling package.
func (s *StageSpec) Validate() error {
	var errs []error
	if s.Dir == "" {
		errs = append(errs, errors.New("Dir is required"))
	}
	if !token.IsIdentifier(s.Package) {
		errs = append(errs, fmt.Errorf("%q is not a package name", s.Package))
	}
	if s.Seats < 1 {
		errs = append(errs, fmt.Errorf("a stage needs at least one seat, not %d", s.Seats))
	}
	if len(errs) > 0 {
		return fmt.Errorf("scaffold: invalid stage: %w", errors.Join(errs...))
	}
	return nil
}

// Path is where the file lands.
func (s *StageSpec) Path() string { return filepath.Join(s.Dir, StageFile) }

// Rel is Path as a person would type it, when the root is known.
func (s *StageSpec) Rel() string { return relTo(s.Root, s.Path()) }

// WriteStage writes the rule set and reports the path.
//
// It refuses a directory that already holds Go files. A stage is the
// unit a session runs, so two rule sets in one package would be two
// games sharing a name — and the generator reads declarations rather
// than files, so it would silently owe codecs to both.
func WriteStage(spec *StageSpec) (string, error) {
	if err := spec.Validate(); err != nil {
		return "", err
	}
	if occupied, err := holdsGo(spec.Dir); err != nil {
		return "", err
	} else if occupied {
		return "", fmt.Errorf("scaffold: %s already holds Go source; a stage is a package of its own, so add will not write into one that is taken", relTo(spec.Root, spec.Dir))
	}
	body, err := execute("stage.go.tmpl", spec)
	if err != nil {
		return "", err
	}
	if body, err = gofmtSource(spec.Path(), body); err != nil {
		return "", err
	}
	if err := os.MkdirAll(spec.Dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(spec.Path(), body, 0o644); err != nil {
		return "", err
	}
	return spec.Path(), nil
}

// holdsGo reports whether a directory already has Go source in it. A
// directory that does not exist holds nothing, which is the ordinary
// case and not an error.
func holdsGo(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}

// StagePackage turns a stage name into a Go package name: lower case,
// letters and digits, since that is the whole of the convention.
func StagePackage(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return -1
		}
	}, name)
}

// relTo names a path the way a person would type it from root.
func relTo(root, path string) string {
	if root == "" {
		return path
	}
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}
