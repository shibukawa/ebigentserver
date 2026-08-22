package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// mapEntryPoints are the cborbind calls that name a world state. A type
// asked for the map shape is one whose members carry their names and
// whose unknown keys a decoder skips — which is concept:cbor-world-profile
// and therefore the thing deltas are computed over. A type asked for the
// array shape is a message, and a message has no baseline to diff against.
var mapEntryPoints = map[string]bool{
	"AppendCBORInMapTo":   true,
	"DecodeCBORInMapFrom": true,
}

// Discover reports the world states declared in dir, which are the types
// its source asks cborbind for a map codec.
//
// The declaration is the call, the same idiom tinybind adopted in v0.5.23:
// there is no separate list to keep in step, and a type nobody encodes as
// a world state gets no delta because nothing asked for one.
//
// It reads syntax rather than types, because at generate time the codecs
// it is about to ask for do not exist yet and the package does not
// compile.
func Discover(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("codegen: %w", err)
	}
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// A generated file names the same types the source does; reading
		// it would be reading this generator's own output back.
		if strings.HasSuffix(name, "_gen.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("codegen: %w", err)
		}
		collectMapTypes(file, seen)
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out, nil
}

// collectMapTypes walks one file for map-shape calls and records the type
// each names.
func collectMapTypes(file *ast.File, seen map[string]bool) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.IndexExpr:
			// DecodeCBORInMapFrom[World](data) — the type argument is
			// written out, which is what makes it discoverable.
			if named(fn.X) && ident(fn.Index) != "" {
				seen[ident(fn.Index)] = true
			}
		case *ast.SelectorExpr:
			// AppendCBORInMapTo(dst, v) infers its type argument, so the
			// value's own type is what names it. A wrapper written over a
			// parameter is the shape the generated one takes.
			if !mapEntryPoints[fn.Sel.Name] || len(call.Args) != 2 {
				return true
			}
			if t := argType(file, call.Args[1]); t != "" {
				seen[t] = true
			}
		}
		return true
	})
}

// named reports whether an expression is one of the map entry points.
func named(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && mapEntryPoints[sel.Sel.Name]
}

// ident is the name of an identifier expression, empty for anything else.
func ident(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// argType resolves the declared type of a value passed to an inferring
// call, by finding the enclosing function's parameter of that name. It is
// deliberately narrow: the wrapper this generator emits, and the one a
// game writes by hand, both pass a parameter straight through.
func argType(file *ast.File, arg ast.Expr) string {
	name := ident(arg)
	if name == "" {
		return ""
	}
	var found string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Type.Params == nil {
			return true
		}
		for _, p := range fn.Type.Params.List {
			for _, id := range p.Names {
				if id.Name == name {
					if t := ident(p.Type); t != "" {
						found = t
					}
				}
			}
		}
		return true
	})
	return found
}
