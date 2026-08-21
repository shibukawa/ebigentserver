// Package confload loads ebigent.toml for whichever process is asking.
//
// It supplies the three things configbind does not: the project locator
// that finds the file by walking upward, the prefix-scoped stray key
// check that decision:one-config-file-many-sections requires, and the
// startup provenance dump.
//
// The layering itself belongs to configbind and is not configurable
// here: default < TOML < environment < CLI, first readable file only,
// never merged (rule:config-precedence-fixed, rule:one-config-file-per-process).
package confload

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shibukawa/tinybind-go/configbind"
)

// FileName is the project configuration file, and the marker that a
// directory is a project root.
const FileName = "ebigent.toml"

// Vendor and Tool place the OS configuration directory fallback, used
// only when no project file is found and no path was given.
const (
	Vendor = "shibukawa"
	Tool   = "ebigent"
)

// ErrNoProject reports that no FileName was found in the starting
// directory or any ancestor.
var ErrNoProject = errors.New("confload: no " + FileName + " found in this directory or any parent")

// FindProjectRoot walks upward from start until it finds FileName and
// returns that directory. An empty start means the working directory.
func FindProjectRoot(start string) (string, error) {
	dir := start
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = wd
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, FileName)); err == nil && !st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoProject
		}
		dir = parent
	}
}

// Options selects the sources for one load.
type Options struct {
	// Owned are the configuration prefixes this process bound. The
	// stray key check applies to these and to nothing else, because
	// every process legitimately sees the sections belonging to the
	// other reader of the same file.
	Owned []string
	// StartDir begins the upward walk for the project file. Empty uses
	// the working directory.
	StartDir string
	// Args are CLI arguments without the program name. Nil uses the
	// process arguments; an empty non-nil slice shuts CLI input off.
	Args []string
	// Environ is the environment as KEY=value. Nil uses the process
	// environment; an empty non-nil slice shuts environment input off.
	Environ []string
	// ExplicitConfigPath forces one file and skips the project walk.
	ExplicitConfigPath string
	// AllowMissingProject keeps a load working outside a project, on
	// defaults, environment, and CLI alone. Commands that create a
	// project set it; commands that operate on one do not.
	AllowMissingProject bool
	// Validate runs after every target is filled, so Load either
	// returns a configuration that is ready to use or an error.
	//
	// Write each entry as a closure over the bound pointer —
	// func() error { return run.Validate() } — never as the method
	// value run.Validate, which copies the struct while it is still
	// empty.
	Validate []func() error
}

// Result is one completed load.
type Result struct {
	// Load is the configbind result, carrying the overlay and
	// provenance.
	Load *configbind.LoadResult
	// ProjectRoot is the directory holding FileName, empty when the
	// load ran without a project.
	ProjectRoot string
}

// Load resolves the project file and fills every bound target. Call it
// once, after every Bind, and never twice in one process.
func Load(opts Options) (*Result, error) {
	res := &Result{}
	lo := configbind.LoadOptions{
		Vendor:             Vendor,
		Tool:               Tool,
		FileName:           FileName,
		Args:               opts.Args,
		Environ:            opts.Environ,
		ExplicitConfigPath: opts.ExplicitConfigPath,
	}
	if opts.ExplicitConfigPath == "" {
		root, err := FindProjectRoot(opts.StartDir)
		switch {
		case err == nil:
			res.ProjectRoot = root
			lo.ExtraConfigReadPaths = []string{filepath.Join(root, FileName)}
		case errors.Is(err, ErrNoProject) && opts.AllowMissingProject:
			// defaults, environment, and CLI still load
		case errors.Is(err, ErrNoProject):
			return nil, err
		default:
			return nil, err
		}
	}

	lr, err := configbind.Load(lo)
	if err != nil {
		return nil, err
	}
	res.Load = lr

	declared, err := DeclaredKeys()
	if err != nil {
		return nil, err
	}
	if stray := StrayKeys(lr.Overlay, opts.Owned, declared); len(stray) > 0 {
		return nil, &StrayKeyError{Path: lr.ConfigPath, Keys: stray}
	}
	for _, check := range opts.Validate {
		if err := check(); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// StrayKeyError reports configuration keys that sit under a prefix this
// process owns but match no declared field. configbind applies an
// unknown TOML key silently, so without this check a misspelled key
// leaves the default in force and says nothing.
type StrayKeyError struct {
	Path string
	Keys []string
}

func (e *StrayKeyError) Error() string {
	where := e.Path
	if where == "" {
		where = "configuration"
	}
	return fmt.Sprintf("confload: %s: unknown %s: %s",
		where, plural("key", len(e.Keys)), strings.Join(e.Keys, ", "))
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// StrayKeys reports keys of o that fall under an owned prefix yet are not
// declared. Keys under any other prefix are ignored: one file serves both
// the toolchain and the artifact, so each reader sees sections it does
// not own and must not object to them.
func StrayKeys(o *configbind.Overlay, owned []string, declared map[string]bool) []string {
	if o == nil || len(owned) == 0 {
		return nil
	}
	var stray []string
	for key, entry := range o.All() {
		if !ownsKey(owned, key) {
			continue
		}
		if !declared[key] {
			stray = append(stray, key)
			continue
		}
		if !entry.IsTables {
			continue
		}
		// Element fields live in their own overlays, keyed relative to
		// the array. A typo there is as silent as one at the top level.
		for i, table := range entry.Tables {
			for sub := range table.All() {
				if declared[key+"[]."+sub] {
					continue
				}
				stray = append(stray, fmt.Sprintf("%s[%d].%s", key, i, sub))
			}
		}
	}
	sort.Strings(stray)
	return stray
}

// ownsKey reports whether key belongs to one of the owned prefixes. A
// prefix matches the whole key or a dot-separated head of it, so owning
// "run" never claims "runtime".
func ownsKey(owned []string, key string) bool {
	for _, p := range owned {
		if key == p || strings.HasPrefix(key, p+".") {
			return true
		}
	}
	return false
}

// DeclaredKeys reports every configuration key registered by the packages
// this binary imports. Element fields of an array of tables appear as
// "prefix.array[].field".
//
// The set is read back from configbind's own TOML scaffold, which is the
// only surface exposing the generated declarations; deriving it that way
// keeps it from drifting the way a hand-kept list would.
func DeclaredKeys() (map[string]bool, error) {
	scaffold, err := configbind.ScaffoldTOML()
	if err != nil {
		return nil, err
	}
	return parseScaffoldKeys(scaffold), nil
}

// parseScaffoldKeys reads table headers and assignments out of the
// scaffold. The scaffold is generated, deterministic, and limited to the
// configuration TOML subset — comments, [table], [[array]], and
// "key = value" — so it needs no general TOML parser.
func parseScaffoldKeys(scaffold string) map[string]bool {
	keys := make(map[string]bool)
	table := ""
	for line := range strings.Lines(scaffold) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if name, ok := strings.CutPrefix(line, "[["); ok {
			if name, ok = strings.CutSuffix(name, "]]"); ok {
				table = strings.TrimSpace(name) + "[]"
				keys[strings.TrimSpace(name)] = true
			}
			continue
		}
		if name, ok := strings.CutPrefix(line, "["); ok {
			if name, ok = strings.CutSuffix(name, "]"); ok {
				table = strings.TrimSpace(name)
			}
			continue
		}
		field, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		field = strings.TrimSpace(field)
		if field == "" || table == "" {
			continue
		}
		keys[table+"."+field] = true
	}
	return keys
}

// WriteProvenance dumps the effective configuration and the layer that
// set each value, which requirement:layered-configuration asks every
// startup to record. Secrets are already masked by configbind.
func WriteProvenance(w io.Writer, r *Result) error {
	if r == nil || r.Load == nil {
		return nil
	}
	for _, e := range r.Load.Provenance() {
		if _, err := fmt.Fprintf(w, "%s = %s (%s)\n", e.Key, e.Value, e.Place); err != nil {
			return err
		}
	}
	return nil
}
