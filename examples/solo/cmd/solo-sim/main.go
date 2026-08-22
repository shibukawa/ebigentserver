// Command solo-sim plays solo with nobody watching: every seat, the
// player's included, is filled by an agent, and every decision is written
// to data:episode-log.
//
// This is the headless half of the loop the game exists to demonstrate.
// Run it to produce a corpus, distil the recorded decisions into
// data:behavior-chip, regenerate an enemy from the approved ones, and run
// it again — see the example's README.
//
// It imports no engine, and that absence is not a build tag: this entry
// simply never mentions one (decision:entry-points-over-build-tags). The
// same rules the window build plays are linked here unchanged.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/examples/solo/game"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

func main() {
	matches := flag.Int("matches", 10, "how many matches to play")
	record := flag.String("record", "", "corpus directory; empty records nothing")
	seed := flag.Uint64("seed", 1, "seed of the first match; later matches add their index")
	paced := flag.Bool("paced", false, "run at the game's tick rate instead of as fast as possible")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Unlimited is what makes a training run finish in seconds; the
	// simulation is identical either way, because the tick loop never
	// reads a clock (concept:game-time-control).
	tc := session.Unlimited
	if *paced {
		tc = session.Paced
	}

	survived := 0
	played := 0
	err := run.Serve(ctx, game.Options(), game.Binding(), run.ServeOptions{
		Matches: *matches,
		Seed:    *seed,
		Time:    tc,
		Record: run.RecordOptions{
			Root: *record,
			// A corpus meant for distillation is worth recording
			// completely: replay_complete keeps the world stream and
			// the checkpoints, so an episode can be replayed and
			// checked rather than only counted.
			Mode: episode.ReplayComplete,
		},
		OnMatch: func(res run.MatchResult) {
			played++
			outcome := "?"
			if sig, ok := res.Outcome(game.Player); ok {
				if sig.Terminal == session.Win {
					survived++
				}
				outcome = sig.Terminal.String()
			}
			fmt.Printf("match %2d  seed %d  %3d ticks  player %s\n",
				res.Index, res.Seed, res.Ticks, outcome)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "solo-sim:", err)
		os.Exit(1)
	}

	fmt.Printf("\n%d/%d matches survived\n", survived, played)
	if *record != "" {
		fmt.Printf("corpus written to %s\n", *record)
		fmt.Println("next: ebigent analyze -corpus " + *record)
	}
}
