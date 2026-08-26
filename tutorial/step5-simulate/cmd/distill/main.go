// Command distill regenerates step 5's compiled agent from the corpus
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
// recipe in the distill package: step 5's is a rotation of three
// opponents, and which of them plays whom is a fact about the game
// rather than a flag (concept:continuous-match-loop lists four pairings
// and the framework picks none of them).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shibukawa/ebigentserver/tutorial/step5-simulate/distill"
)

func main() {
	flag.Parse()

	// Two bots, one command. The baseline is step 4's recipe rebuilt
	// here, and it is regenerated alongside the rotated one so that the
	// swap in main.go has two reproducible sides.
	base, err := distill.CompileBaseline()
	if err != nil {
		fatal(err)
	}
	if err := base.Write("distill/gen"); err != nil {
		fatal(err)
	}
	report("gen.Distilled (random opponent)", distill.BaselineMatches, base, "distill/gen")

	rot, err := distill.Compile()
	if err != nil {
		fatal(err)
	}
	if err := rot.WriteAs("distill/genrotated", distill.RotatedSpec(), distill.RotatedTestSpec()); err != nil {
		fatal(err)
	}
	report("genrotated.Rotated (round_robin)", distill.CorpusMatches, rot, "distill/genrotated")
}

// report prints one bot's summary. The coverage is the number worth
// printing, and it is not the one the miner reports: Propose calls
// every record covered while a third of them are explained by rules
// nobody approved.
func report(name string, matches int, c *distill.Compiled, out string) {
	fmt.Printf("%s: %d games, %d decisions, %d chips → %s\n",
		name, matches, len(c.Records), len(c.Library.Approved()), out)
	fmt.Printf("  %.1f%% of the recorded decisions are explained by an approved chip\n",
		100*float64(distill.Covered(c.Library, c.Records))/float64(len(c.Records)))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "distill:", err)
	os.Exit(1)
}
