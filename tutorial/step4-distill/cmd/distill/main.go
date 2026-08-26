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
	"path/filepath"

	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill"
)

func main() {
	corpus := flag.String("corpus", env("EBIGENT_CORPUS", "corpus"), "episode corpus root")
	out := flag.String("out", "", "where the generated package is written; empty picks by target")
	// Two targets, one pipeline. "bot" is the canonical recipe ebigent
	// distill runs; "you" mines a curated human corpus into the
	// genhuman package, so your copy and the bot's live side by side.
	target := flag.String("target", "bot", "bot mines the X seat into distill/gen; you mines every seat into distill/genhuman")
	flag.Parse()

	compile, read, dir := distill.CompileFrom, distill.Corpus, "distill/gen"
	if *target == "you" {
		compile, read, dir = distill.CompileYours, distill.YourCorpus, "distill/genhuman"
	} else if *target != "bot" {
		fatal(fmt.Errorf("unknown --target %q; give bot or you", *target))
	}
	if *out == "" {
		*out = dir
	}

	c, err := compile(*corpus)
	if err != nil {
		fatal(err)
	}
	if *target == "you" {
		err = c.WriteAs(*out, distill.HumanSpec(), distill.HumanTestSpec())
	} else {
		err = c.Write(*out)
	}
	if err != nil {
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

	// A curated corpus keeps a holdout beside its train side, and that
	// is the honest half of the report: play the miner never saw.
	hold, ok, err := distill.EvaluateHoldout(*corpus, c, read)
	if err != nil {
		fatal(err)
	}
	if ok {
		fmt.Printf("holdout: %d decisions answered as recorded, %d answered differently, %d silent\n",
			len(hold.Covered), len(hold.Misplayed), len(hold.Silent))
		fmt.Printf("silent situations written to %s\n",
			filepath.Join(filepath.Dir(filepath.Clean(*corpus)), "gaps.jsonl"))
	}
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
