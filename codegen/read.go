// Package codegen reads a game's wire types and emits what the framework
// needs from them that no library supplies any more.
//
// tinybind-go generates the codecs (requirement:cborbind-migration): a
// call to its entry point is the ask, and it emits the array or map shape
// the call named. What it does not generate, since v0.5.21 removed it, is
// the delta half of concept:state-synchronization — the diff between two
// world versions, the patch that puts one back, and the encoding that
// carries only what changed. decision:framework-side-delta-generation
// stops being a choice at that point and becomes this package.
package codegen

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Kind is how one field is carried, which decides every line emitted for
// it: the comparison in the diff, the assignment in the patch, and the
// CBOR primitive in the codec.
type Kind uint8

const (
	// KindUint is an unsigned integer of any width, including a named
	// type over one.
	KindUint Kind = iota
	// KindInt is a signed integer, including the fixed-point scales of
	// decision:fixed-point-numeric-representation, which are named types
	// over int64 and travel as integers.
	KindInt
	// KindBool is a boolean.
	KindBool
	// KindBytes is a slice of unsigned bytes, carried whole.
	KindBytes
	// KindStructSlice is a slice of structs, carried whole and encoded
	// element by element through each one's generated codec.
	KindStructSlice
)

// Field is one member of a world state, resolved far enough to emit code
// for it.
type Field struct {
	// Name is the Go field name.
	Name string
	// Type is the type as written, so the emitted delta declares the
	// same one.
	Type string
	// Kind decides how it is compared, assigned, and encoded.
	Kind Kind
	// Elem is the element type of a struct slice, empty otherwise.
	Elem string
}

// Struct is one world state type the framework generates a delta for.
type Struct struct {
	// Name is the type name.
	Name string
	// Fields are its members in declaration order, which is the order
	// the presence bits are assigned in — so moving a field moves the
	// wire and therefore the schema version with it.
	Fields []Field
}

// ErrUnsupported reports a field the delta generator cannot carry. It
// names the type and the field rather than skipping either, because a
// field silently left out of a delta is a world that stops updating.
type ErrUnsupported struct {
	Struct, Field, Type string
}

func (e *ErrUnsupported) Error() string {
	return fmt.Sprintf("codegen: %s.%s is %s, which a delta cannot carry", e.Struct, e.Field, e.Type)
}

// Read loads dir and resolves the named types. Names are read in the
// order given, and the result follows it, so the emitted file is stable
// against the order the loader happens to return packages in.
func Read(dir string, names []string) (pkg string, out []Struct, err error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  dir,
	}
	loaded, err := packages.Load(cfg, ".")
	if err != nil {
		return "", nil, fmt.Errorf("codegen: %w", err)
	}
	if len(loaded) != 1 {
		return "", nil, fmt.Errorf("codegen: %s holds %d packages, want one", dir, len(loaded))
	}
	p := loaded[0]
	if len(p.Errors) > 0 {
		// A package that does not type-check cannot be read for its
		// field types, and guessing from the syntax would emit a codec
		// for a shape the compiler never agreed to.
		return "", nil, fmt.Errorf("codegen: %s does not compile: %v", dir, p.Errors[0])
	}

	for _, name := range names {
		obj := p.Types.Scope().Lookup(name)
		if obj == nil {
			return "", nil, fmt.Errorf("codegen: %s declares no type %s", p.PkgPath, name)
		}
		st, ok := obj.Type().Underlying().(*types.Struct)
		if !ok {
			return "", nil, fmt.Errorf("codegen: %s.%s is not a struct", p.PkgPath, name)
		}
		s, err := readStruct(name, st)
		if err != nil {
			return "", nil, err
		}
		out = append(out, s)
	}
	return p.Name, out, nil
}

// readStruct resolves every exported field. An unexported one is skipped
// rather than refused: it never reached the wire in the first place, so a
// delta that omits it carries exactly what the codec does.
func readStruct(name string, st *types.Struct) (Struct, error) {
	s := Struct{Name: name}
	for i := range st.NumFields() {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		kind, elem, err := classify(f.Type())
		if err != nil {
			return Struct{}, &ErrUnsupported{Struct: name, Field: f.Name(), Type: f.Type().String()}
		}
		s.Fields = append(s.Fields, Field{
			Name: f.Name(),
			Type: shortType(f.Type()),
			Kind: kind,
			Elem: elem,
		})
	}
	if len(s.Fields) == 0 {
		return Struct{}, fmt.Errorf("codegen: %s has no exported field to diff", name)
	}
	if len(s.Fields) > 64 {
		// The presence mask is one uint64, and widening it would move
		// every existing wire format.
		return Struct{}, fmt.Errorf("codegen: %s has %d fields, past the 64 a presence mask holds", name, len(s.Fields))
	}
	return s, nil
}

// classify decides how a field travels. Floats are refused here rather
// than at review time: rule:no-float-in-simulation keeps reals out of the
// simulation path, and the profile that used to refuse them at the
// encoder is gone (requirement:cborbind-migration), so this is now the
// only gate.
func classify(t types.Type) (Kind, string, error) {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		info := u.Info()
		switch {
		case info&types.IsBoolean != 0:
			return KindBool, "", nil
		case info&types.IsUnsigned != 0:
			return KindUint, "", nil
		case info&types.IsInteger != 0:
			return KindInt, "", nil
		}
		return 0, "", fmt.Errorf("unsupported basic type %s", u)
	case *types.Slice:
		e := u.Elem()
		if b, ok := e.Underlying().(*types.Basic); ok {
			if b.Kind() == types.Uint8 {
				return KindBytes, "", nil
			}
			return 0, "", fmt.Errorf("unsupported slice element %s", b)
		}
		if _, ok := e.Underlying().(*types.Struct); ok {
			return KindStructSlice, shortType(e), nil
		}
		return 0, "", fmt.Errorf("unsupported slice element %s", e)
	}
	return 0, "", fmt.Errorf("unsupported type %s", t)
}

// shortType spells a type the way the generated file, which sits in the
// same package, has to spell it.
func shortType(t types.Type) string {
	s := types.TypeString(t, func(p *types.Package) string { return "" })
	return strings.ReplaceAll(s, ".", "")
}

// SortedNames returns names in a stable order, for a caller that
// discovered them from a map.
func SortedNames(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}
