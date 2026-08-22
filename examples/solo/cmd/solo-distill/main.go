// Command solo-distill turns recorded enemy play into Go source.
//
// It plays a corpus, mines each enemy kind's decisions into
// data:behavior-chip, approves the clean rules, and writes the compiled
// decision list plus fixture tests under distill/gen. Running it again
// after changing the rules, the sample enemies, or the vocabulary is how
// the loop repeats — and the generated files are committed, so what
// changed is a diff a person can read (decision:behavior-tree-compiled-to-go).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/ebigentserver/examples/solo/distill"
)

func main() {
	matches := flag.Int("matches", 24, "matches to record before mining")
	seed := flag.Uint64("seed", 1, "seed of the first match")
	corpus := flag.String("corpus", "", "corpus directory; empty uses a temporary one")
	out := flag.String("out", "examples/solo/distill/gen", "where the generated packages are written")
	flag.Parse()

	root := *corpus
	if root == "" {
		dir, err := os.MkdirTemp("", "solo-corpus-")
		if err != nil {
			fatal(err)
		}
		defer os.RemoveAll(dir)
		root = dir
	}

	if err := distill.Play(context.Background(), root, *matches, *seed); err != nil {
		fatal(err)
	}
	fmt.Printf("recorded %d matches into %s\n", *matches, root)

	for _, kind := range distill.Kinds() {
		c, err := distill.Compile(root, kind)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", kind.Name, err))
		}
		dir := filepath.Join(*out, kind.Package)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "agent_gen.go"), c.Agent, 0o644); err != nil {
			fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "fixtures_gen_test.go"), c.Tests, 0o644); err != nil {
			fatal(err)
		}
		if err := c.Library.Save(filepath.Join(dir, "chips.json")); err != nil {
			fatal(err)
		}
		fmt.Printf("%-8s %d decisions → %d chips → %s\n",
			kind.Name, len(c.Records), len(c.Library.Approved()), dir)
		for _, chip := range c.Library.Approved() {
			fmt.Printf("    %-34s → %-11s coverage %d\n", chip.Condition, chip.Action, chip.Coverage)
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "solo-distill:", err)
	os.Exit(1)
}
