// Command regen runs the codec half of `ebigent generate` over this
// repository, which is a framework rather than a game and so has no
// ebigent.toml of its own to run the verb from.
//
// It is the same pipeline in the same order — ask, generate, verify,
// delta — so the files committed under samples/, examples/, and
// tutorial/ are the files a game gets.
//
//	go run ./internal/cmd/regen
//
// The tutorial steps are their own modules and each has an ebigent.toml,
// so `ebigent generate` covers them on its own; this walks the tree
// anyway, and reproduces what that verb writes.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/shibukawa/ebigentserver/codegen"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	asks, err := codegen.Stages(root)
	check(err)

	dirs := make([]string, 0, len(asks))
	for dir := range asks {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	// The call is the ask, so the asks have to be on disk before the
	// codec generator reads the package.
	for _, dir := range dirs {
		pkg, err := codegen.PackageName(dir)
		check(err)
		check(os.WriteFile(filepath.Join(dir, "wire_gen.go"), codegen.EmitAsks(pkg, asks[dir]), 0o644))
	}
	ready := dirs[:0:0]
	for _, dir := range dirs {
		check(codecs(dir))
		kept, problems := codegen.CheckAsks(dir, asks[dir])
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "%s: %v\n\n", dir, p)
		}
		if len(problems) > 0 {
			// Rewrite without the withdrawn asks and generate again, so
			// nothing half-written is left where a codec should be.
			asks[dir] = kept
			if len(kept) == 0 {
				remove(dir, "wire_gen.go", "tinybind_gen.go")
			} else {
				pkg, err := codegen.PackageName(dir)
				check(err)
				check(os.WriteFile(filepath.Join(dir, "wire_gen.go"), codegen.EmitAsks(pkg, kept), 0o644))
				check(codecs(dir))
			}
		}
		if len(kept) > 0 {
			ready = append(ready, dir)
		}
	}
	dirs = ready

	// Deltas cover what the declarations asked for plus what a package
	// still asks for by hand — a per-role view a game synchronises
	// without it being the rule set's world.
	for _, dir := range append(dirs, extra(root, dirs)...) {
		names, err := codegen.Discover(dir)
		check(err)
		names = codegen.SortedNames(append(names, codegen.AskedWorlds(asks[dir])...))
		if len(names) == 0 {
			continue
		}
		pkg, structs, err := codegen.Read(dir, names)
		check(err)
		src, err := codegen.Emit(pkg, structs)
		check(err)
		check(os.WriteFile(filepath.Join(dir, "delta_gen.go"), src, 0o644))
		check(os.WriteFile(filepath.Join(dir, "schema_gen.go"), codegen.EmitVersion(pkg, structs), 0o644))
		fmt.Println(dir, names, codegen.Version(structs))
	}
}

// remove deletes generated files that must not be left behind.
func remove(dir string, files ...string) {
	for _, f := range files {
		if err := os.Remove(filepath.Join(dir, f)); err != nil && !os.IsNotExist(err) {
			check(err)
		}
	}
}

// extra are the packages no rule set names but that still ask by hand.
// examples/phase0 is the one here: it predates the runtime, so it has a
// world and no stage to declare it.
func extra(root string, done []string) []string {
	seen := map[string]bool{}
	for _, d := range done {
		seen[d] = true
	}
	var out []string
	for _, d := range []string{filepath.Join(root, "examples", "phase0", "msg")} {
		if !seen[d] {
			out = append(out, d)
		}
	}
	return out
}

// codecs runs the codec generator in one package.
func codecs(dir string) error {
	cmd := exec.Command("go", "tool", "tinybind-gen", "generate", "-openapi=false")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w\n%s", dir, err, out)
	}
	return nil
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
