// Command distill regenerates step 4's compiled agent from the corpus
// recipe the committed sources came from.
//
// `ebigent distill` runs it: behavior.distill in ebigent.toml points
// here, and the toolchain spawns it the way `ebigent build` spawns
// `go build`. It has to live in the project rather than in the tool
// because a data:derived-predicate is a Go function over a sight, and a
// compiled ebigent cannot receive one from a module it was built
// without.
//
// It takes no corpus argument. The corpus is recorded here, from the
// recipe in the distill package, because what step 4 is measuring is
// what a corpus of a given size can and cannot teach — and a size
// supplied on the command line would be a second place for the recipe to
// live (see distill.Compiled).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill"
)

func main() {
	out := flag.String("out", "distill/gen", "where the generated package is written")
	flag.Parse()

	c, err := distill.Compile()
	if err != nil {
		fatal(err)
	}
	if err := c.Write(*out); err != nil {
		fatal(err)
	}

	approved := c.Library.Approved()
	fmt.Printf("%d games, %d decisions, %d chips → %s\n",
		distill.CorpusMatches, len(c.Records), len(approved), *out)
	// The coverage is the number worth printing, and it is not the one
	// the miner reports: Propose calls every record covered while a
	// third of them are explained by rules nobody approved.
	fmt.Printf("%.1f%% of the recorded decisions are explained by an approved chip\n",
		100*float64(distill.Covered(c.Library, c.Records))/float64(len(c.Records)))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "distill:", err)
	os.Exit(1)
}
