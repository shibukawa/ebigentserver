package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// generatedCodecs is the file the codec generator writes.
const generatedCodecs = "tinybind_gen.go"

// CheckAsks separates the codecs that were generated whole from the ones
// that were not, and says what is wrong with each of the latter.
//
// It exists because the codec generator reports neither. A type it cannot
// handle produces no codec and no error, and the package is reported as
// having nothing to generate — which names neither the type nor the
// reason. Worse, a type it can *partly* handle produces a codec that
// compiles, round trips, and silently leaves members out: a world
// synchronised through one would keep its board and lose its RNG seed,
// and the first symptom is two peers diverging.
//
// So the framework checks the output against the source. Comparing
// members is enough to catch both: the container says what the codec
// writes, and the struct says what it should.
//
// A failing ask is withdrawn rather than fatal. Withdrawing leaves the
// package exactly as it was before anything asked — no codec, and none
// that lies — while the returned problem names the type and the members,
// which is the diagnosis the generator does not give. Making it fatal
// would stop every project today, because a world holds a tick or a seat
// or a fixed-point number and none of those are carried yet
// (requirement:stage-declares-its-wire).
func CheckAsks(dir string, asks []Ask) (ok []Ask, problems []error) {
	if len(asks) == 0 {
		return nil, nil
	}
	members, err := declaredMembers(dir)
	if err != nil {
		return nil, []error{err}
	}
	emitted, err := emittedMembers(dir)
	if err != nil {
		return nil, []error{err}
	}
	for _, a := range asks {
		if err := checkAsk(dir, a, members, emitted); err != nil {
			problems = append(problems, err)
			continue
		}
		ok = append(ok, a)
	}
	return ok, problems
}

// checkAsk is one asked-for codec against what was generated for it.
func checkAsk(dir string, a Ask, members map[string][]member, emitted map[string][]string) error {
	want, declared := members[a.Type]
	if !declared {
		return fmt.Errorf("%s is the %s of a rule set but %s declares no such struct", a.Type, a.Position, dir)
	}
	got, generated := emitted[a.Type]
	if !generated {
		return fmt.Errorf("no codec for %s, the %s of a rule set: the generator refused the whole type.\n"+
			"  It refuses a collection of a named element type — []Mark or [9]Mark, where []uint8 is carried.\n"+
			"  Members: %s", a.Type, a.Position, names(want))
	}
	if missing := missingMembers(want, got, a.Shape); len(missing) > 0 {
		return fmt.Errorf("no codec for %s, the %s of a rule set: the generator carried %d of its %d members and left out %s.\n"+
			"  It leaves out a member whose type is a named type from another package — session.Tick, fixmath.F64.\n"+
			"  A codec missing those would lose them on every send, so it is not kept.",
			a.Type, a.Position, len(got), len(want), names(missing))
	}
	return nil
}

// member is one struct field: the name to report it by, and the key the
// codec writes it under.
type member struct {
	Name string
	Key  string
}

// names renders members for a diagnostic.
func names(ms []member) string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return strings.Join(out, ", ")
}

// missingMembers names what the codec left out. A map codec writes its
// keys, so the answer is exact; an array codec writes positions only, so
// the count is all there is and the whole member list is reported.
func missingMembers(want []member, got []string, shape Shape) []member {
	if shape == Map {
		have := map[string]bool{}
		for _, k := range got {
			have[k] = true
		}
		var out []member
		for _, w := range want {
			if !have[w.Key] {
				out = append(out, w)
			}
		}
		return out
	}
	if len(got) == len(want) {
		return nil
	}
	return want
}

// declaredMembers are the exported members of every struct in dir, by
// type name, in declaration order.
func declaredMembers(dir string) (map[string][]member, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("codegen: %w", err)
	}
	out := map[string][]member{}
	fset := token.NewFileSet()
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") || strings.HasSuffix(n, "_gen.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("codegen: %w", err)
		}
		collectMembers(f, out)
	}
	return out, nil
}

// collectMembers records one file's struct members.
func collectMembers(f *ast.File, out map[string][]member) {
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Assign.IsValid() {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			var ms []member
			for _, fld := range st.Fields.List {
				key, ok := wireKey(fld)
				if !ok {
					continue
				}
				for _, id := range fld.Names {
					if !id.IsExported() {
						continue
					}
					k := key
					if k == "" {
						k = strings.ToLower(id.Name)
					}
					ms = append(ms, member{Name: id.Name, Key: k})
				}
			}
			out[ts.Name.Name] = ms
		}
	}
}

// wireKey is the key a member is written under: its json tag, or its
// lowercased name when it has none. It reports false for a member the
// codec is not expected to carry at all.
func wireKey(f *ast.Field) (string, bool) {
	if f.Tag == nil {
		return "", true
	}
	tag, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return "", true
	}
	name, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ",")
	if name == "-" {
		return "", false
	}
	return name, true
}

// emittedMembers are the members each generated codec actually writes: the
// text keys for a map, and one entry per position for an array.
func emittedMembers(dir string) (map[string][]string, error) {
	path := filepath.Join(dir, generatedCodecs)
	src, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]string{}, nil
		}
		return nil, fmt.Errorf("codegen: %w", err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("codegen: %w", err)
	}
	out := map[string][]string{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil {
			continue
		}
		typeName, shape, ok := codecFuncName(fn.Name.Name)
		if !ok {
			continue
		}
		out[typeName] = writtenMembers(fn, shape)
	}
	return out, nil
}

// codecFuncName reads the type and shape out of an emitted encoder's
// name, which the generator spells append<Type>CBOR<Shape>.
func codecFuncName(name string) (string, Shape, bool) {
	for suffix, shape := range map[string]Shape{"CBORMap": Map, "CBORArray": Array} {
		if rest, ok := strings.CutPrefix(name, "append"); ok {
			if t, ok := strings.CutSuffix(rest, suffix); ok && t != "" {
				return t, shape, true
			}
		}
	}
	return "", 0, false
}

// writtenMembers counts what one encoder writes: the keys of a map, or
// the width of the array header.
func writtenMembers(fn *ast.FuncDecl, shape Shape) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch {
		case shape == Map && sel.Sel.Name == "AppendText" && len(call.Args) == 2:
			// Only the encoder's own keys count. A nested value writes
			// its keys inside a loop, and those belong to its own type.
			if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if k, err := strconv.Unquote(lit.Value); err == nil {
					out = append(out, k)
				}
			}
		case shape == Array && sel.Sel.Name == "AppendArrayHeader" && len(call.Args) == 2 && out == nil:
			if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.INT {
				if n, err := strconv.Atoi(lit.Value); err == nil {
					out = make([]string, n)
				}
			}
		}
		return true
	})
	if out == nil {
		out = []string{}
	}
	return out
}

// PackageName reads the package clause of a directory, so a generated
// file declares the same one as the source beside it.
func PackageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("codegen: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err != nil {
			return "", fmt.Errorf("codegen: %w", err)
		}
		return f.Name.Name, nil
	}
	return "", fmt.Errorf("codegen: %s holds no Go source", dir)
}
