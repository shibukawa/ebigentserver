package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/ebigentserver/codegen"
)

func main() {
	for _, dir := range os.Args[1:] {
		names, err := codegen.Discover(dir)
		check(err)
		pkg, structs, err := codegen.Read(dir, names)
		check(err)
		src, err := codegen.Emit(pkg, structs)
		check(err)
		check(os.WriteFile(filepath.Join(dir, "delta_gen.go"), src, 0o644))
		check(os.WriteFile(filepath.Join(dir, "schema_gen.go"), codegen.EmitVersion(pkg, structs), 0o644))
		fmt.Println(dir, names, codegen.Version(structs))
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
