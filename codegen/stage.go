package codegen

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ruleSetNames are the contracts a stage asserts it satisfies. Both name
// the same three positions; the tick variant only adds a per-tick step.
var ruleSetNames = map[string]bool{
	"StageRuleSet":     true,
	"TickStageRuleSet": true,
}

// Shape is the container a codec uses, and it is a consequence of the
// position rather than a preference (requirement:stage-declares-its-wire).
type Shape uint8

const (
	// Array is positional and carries no member names. Both ends are
	// rebuilt together, which suits a message going one way once.
	Array Shape = iota
	// Map carries text keys and a decoder skips one it does not know,
	// so the two ends may ship apart. It is also the only shape a delta
	// can be computed over, because a delta names what changed.
	Map
)

// Ask is one codec the generator owes a package.
type Ask struct {
	// Type is the named type, as written in the package that declares it.
	Type string
	// Shape is the container it is encoded in.
	Shape Shape
	// Position is which of the rule set's three type parameters asked
	// for it, for a diagnostic that can point back at the declaration.
	Position string
}

// Stages reports every codec the rule set declarations under root ask
// for, grouped by the directory of the package that declares each type.
//
// The rule set declaration is the one place a stage meets the framework,
// so it is the only place generation reads (requirement:stage-declares-
// its-wire). A game states `var _ session.TickStageRuleSet[W, A, S]` to
// have the compiler check its rules against the contract — an assertion
// it writes anyway — and that assertion already names everything that
// travels. There is nothing else to declare and nothing to keep in step.
//
// It reads syntax rather than types, because at generate time the codecs
// it is about to ask for do not exist yet and the package does not
// compile.
//
// The sight position is not asked for yet. Its blocker is upstream and
// recorded in requirement:stage-declares-its-wire: the codec generator
// carries neither a named scalar from another package nor a slice of a
// named one, and a sight names seats and carries the framework's own
// evaluation signal, so every game's sight hits both.
func Stages(root string) (map[string][]Ask, error) {
	mod, err := moduleOf(root)
	if err != nil {
		return nil, err
	}
	out := map[string][]Ask{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		return collectStage(root, mod, p, out)
	})
	if err != nil {
		return nil, fmt.Errorf("codegen: %w", err)
	}
	for dir := range out {
		out[dir] = dedupeAsks(out[dir])
	}
	return out, nil
}

// skipDir names the directories a walk never descends into.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "bin" || name == "node_modules"
}

// collectStage reads one file for rule set assertions and records what
// each one asks for.
func collectStage(root, mod, file string, out map[string][]Ask) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	for _, decl := range f.Decls {
		args := assertedRuleSet(decl)
		if args == nil {
			continue
		}
		for i, pos := range []string{"world", "action"} {
			d, name, ok := resolveType(root, mod, dir, f, args[i])
			if !ok {
				return fmt.Errorf("%s: the %s position of the rule set is %s, which no package here declares",
					rel(root, file), pos, exprString(args[i]))
			}
			shape := Array
			if pos == "world" {
				shape = Map
			}
			out[d] = append(out[d], Ask{Type: name, Shape: shape, Position: pos})
		}
	}
	return nil
}

// assertedRuleSet returns the three type arguments of a rule set
// assertion, or nil for any other declaration.
//
// The assertion is what it matches — `var _ session.StageRuleSet[…] = x{}`
// — rather than any mention of the interface. A struct field holding one,
// and the interface's own definition, name the same three parameters
// without being a stage: only a game asserting its own rules is declaring
// one.
func assertedRuleSet(decl ast.Decl) []ast.Expr {
	gen, ok := decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		return nil
	}
	for _, spec := range gen.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "_" || vs.Type == nil {
			continue
		}
		idx, ok := vs.Type.(*ast.IndexListExpr)
		if !ok || len(idx.Indices) != 3 {
			continue
		}
		sel, ok := idx.X.(*ast.SelectorExpr)
		if !ok || !ruleSetNames[sel.Sel.Name] {
			continue
		}
		return idx.Indices
	}
	return nil
}

// resolveType follows one type argument to the directory and name of the
// type it denotes, through a local alias if there is one.
//
// A game may write the type where its rules are and alias it to the
// package the wire types live in. Both spellings name the same type, so
// both have to arrive at the same package.
func resolveType(root, mod, dir string, f *ast.File, e ast.Expr) (string, string, bool) {
	switch t := e.(type) {
	case *ast.Ident:
		// A local name, unless it is an alias to somewhere else.
		if target := aliasTarget(dir, t.Name); target != nil {
			return resolveType(root, mod, dir, target.file, target.expr)
		}
		return dir, t.Name, true
	case *ast.SelectorExpr:
		pkg, ok := t.X.(*ast.Ident)
		if !ok {
			return "", "", false
		}
		d, ok := importDir(root, mod, f, pkg.Name)
		if !ok {
			return "", "", false
		}
		return d, t.Sel.Name, true
	}
	return "", "", false
}

// alias is a type declared as an alias, with the file it was written in
// so its own imports resolve.
type alias struct {
	file *ast.File
	expr ast.Expr
}

// aliasTarget finds `type Name = something` in dir, or nil when Name is a
// type of its own.
func aliasTarget(dir, name string) *alias {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, 0)
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if ok && ts.Assign.IsValid() && ts.Name.Name == name {
					return &alias{file: f, expr: ts.Type}
				}
			}
		}
	}
	return nil
}

// importDir maps a package identifier used in f to the directory it was
// imported from, for imports inside this module.
func importDir(root, mod string, f *ast.File, name string) (string, bool) {
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		local := path.Base(p)
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local != name {
			continue
		}
		if p != mod && !strings.HasPrefix(p, mod+"/") {
			return "", false // another module: not ours to generate for
		}
		return filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(p, mod), "/"))), true
	}
	return "", false
}

// moduleOf reads the module path out of root's go.mod.
func moduleOf(root string) (string, error) {
	src, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	for line := range strings.Lines(string(src)) {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(after), nil
		}
	}
	return "", fmt.Errorf("codegen: %s/go.mod names no module", root)
}

// dedupeAsks collapses repeats and orders what is left, so a package
// asked for the same type by two stages is generated once and the emitted
// file does not move when a stage is added elsewhere.
func dedupeAsks(asks []Ask) []Ask {
	seen := map[string]Ask{}
	for _, a := range asks {
		// A type in both a world and an action position would be a
		// mistake worth surfacing, but the map shape is the safe answer
		// while it stands: it carries names and it can be diffed.
		if prev, ok := seen[a.Type]; ok && prev.Shape == Map {
			continue
		}
		seen[a.Type] = a
	}
	out := make([]Ask, 0, len(seen))
	for _, a := range seen {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// rel is a path relative to root for a message, falling back to the path
// itself.
func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return r
	}
	return p
}

// exprString renders a type expression for a diagnostic.
func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	}
	return "an unnamed type"
}

// EmitAsks renders the file that asks for a package's codecs.
//
// Since v0.5.23 there is no declaration form: calling an entry point is
// the ask, and which entry point decides the container. That makes the
// ask a mechanical consequence of the rule set declaration, which is why
// it is generated here instead of written into a game's message package
// by hand — the shapes are not a choice a game gets to make differently
// for a world than for an action.
//
// The wrappers are never called. They exist to be read by the codec
// generator, and the linker drops them from the artifact.
func EmitAsks(pkg string, asks []Ask) []byte {
	src := askSource(pkg, asks)
	// The emitter writes one line per ask; gofmt decides where the long
	// ones break, so the committed file is the one a reader would write.
	out, err := format.Source(src)
	if err != nil {
		return src
	}
	return out
}

// askSource renders the asks before formatting.
func askSource(pkg string, asks []Ask) []byte {
	var b strings.Builder
	b.WriteString(`// Code generated by ebigent generate; DO NOT EDIT.
//
// These are the codecs this package's types were asked for, and the ask
// is the call: since tinybind v0.5.23 there is no declaration to write,
// and the entry point named decides the container
// (requirement:cborbind-migration).
//
// What is asked for comes from the rule set declaration and nowhere else
// (requirement:stage-declares-its-wire). A world is a map, because only a
// map can be diffed and because the two ends may ship apart. An action is
// an array, because it is one small message with no baseline to diff
// against and both ends are rebuilt together.
//
// Nothing calls these. They are what the codec generator reads, and the
// linker drops them.

package `)
	b.WriteString(pkg)
	b.WriteString("\n\nimport \"github.com/shibukawa/tinybind-go/cborbind\"\n")
	for _, a := range asks {
		shape := "Array"
		if a.Shape == Map {
			shape = "Map"
		}
		fmt.Fprintf(&b, `
// ask%[1]s is the %[2]s position of a rule set: the whole value in the
// %[3]s shape.
func ask%[1]s(dst []byte, v %[1]s) []byte { return cborbind.AppendCBORIn%[4]sTo(dst, v) }

func askDecode%[1]s(data []byte) (%[1]s, error) { return cborbind.DecodeCBORIn%[4]sFrom[%[1]s](data) }
`, a.Type, a.Position, strings.ToLower(shape), shape)
	}
	return []byte(b.String())
}

// AskedWorlds are the types asked for in the map shape, which are the
// ones a delta is generated for.
func AskedWorlds(asks []Ask) []string {
	var out []string
	for _, a := range asks {
		if a.Shape == Map {
			out = append(out, a.Type)
		}
	}
	sort.Strings(out)
	return out
}
