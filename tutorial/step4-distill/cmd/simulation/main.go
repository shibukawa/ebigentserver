// Command simulation plays step 4's corpus with nobody watching.
//
// `ebigent simulate` builds and runs it, handing down the match count,
// the seed, and where the episodes go — all three are [run.episode]
// values, so the same binary run by hand does the same thing.
//
// What it does not take from the command line is who plays whom. That is
// a fact about the game: the bot belongs on X because its decisions are
// what step 4 mines, and the coin belongs on O because a deterministic
// opponent would make eight hundred matches one match written down eight
// hundred times.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/shibukawa/ebigentserver/config/confload"
	"github.com/shibukawa/ebigentserver/config/runconf"
	"github.com/shibukawa/ebigentserver/tutorial/step4-distill/distill"
)

func main() {
	cfg := runconf.Bind()
	if _, err := confload.Load(confload.Options{
		Owned:    runconf.Prefixes(),
		Validate: []func() error{func() error { return cfg.Validate() }},
	}); err != nil {
		fatal(err)
	}
	ep := cfg.Episode
	if ep.Root == "" {
		fatal(errors.New("no corpus root; run `ebigent simulate`, or set run.episode.root"))
	}
	if err := distill.Record(ep.Root, ep.Matches, uint64(ep.Seed)); err != nil {
		fatal(err)
	}
	fmt.Printf("%d matches from seed %d → %s\n", ep.Matches, ep.Seed, ep.Root)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "simulation:", err)
	os.Exit(1)
}
