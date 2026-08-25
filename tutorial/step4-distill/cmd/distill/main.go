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
// It reads what `ebigent simulate` wrote. The two verbs are the two
// halves of the cycle and they meet at the corpus root: simulate fills
// it, distill mines it, and neither knows anything about the other.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill"
)

func main() {
	corpus := flag.String("corpus", env("EBIGENT_CORPUS", "corpus"), "episode corpus root")
	out := flag.String("out", "distill/gen", "where the generated package is written")
	flag.Parse()

	c, err := distill.CompileFrom(*corpus)
	if err != nil {
		fatal(err)
	}
	if err := c.Write(*out); err != nil {
		fatal(err)
	}

	approved := c.Library.Approved()
	fmt.Printf("%d decisions from %s, %d chips → %s\n",
		len(c.Records), *corpus, len(approved), *out)
	// The coverage is the number worth printing, and it is not the one
	// the miner reports: Propose calls every record covered while a
	// third of them are explained by rules nobody approved.
	fmt.Printf("%.1f%% of the recorded decisions are explained by an approved chip\n",
		100*float64(distill.Covered(c.Library, c.Records))/float64(len(c.Records)))
}

// env reads what the toolchain sets, falling back to the configured
// default so this entry also runs on its own.
func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "distill:", err)
	os.Exit(1)
}
