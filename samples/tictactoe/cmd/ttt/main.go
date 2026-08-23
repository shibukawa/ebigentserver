// Command ttt plays sample:tic-tac-toe in the terminal. Which controller
// kind occupies each slot is a launch flag — the review test of
// decision:no-ai-game-mode: a flag selecting controllers is fine, a
// conditional on controller kind inside game logic is not (and there is
// none; see the ttt package).
//
//	ttt              # human plays X against the bot (rendered as Y)
//	ttt -x=bot -y=human
//	ttt -x=bot -y=bot
//
// Presentation (board rendering, input parsing) lives here in the entry
// point, matching decision:entry-points-over-build-tags. A human at a
// terminal and the trivial bot go through the identical
// session.Agent interface.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shibukawa/ebigentserver/samples/tictactoe/ttt"
	"github.com/shibukawa/ebigentserver/session"
)

func main() {
	xKind := flag.String("x", "human", "controller for X: human or bot")
	yKind := flag.String("y", "bot", "controller for Y (the O slot): human or bot")
	flag.Parse()

	s, err := session.New(session.Config[ttt.State, ttt.Move, ttt.Observation]{
		ID:        "ttt-cli",
		Slots:     ttt.Slots(),
		RuleSet:   ttt.RuleSet{},
		Validator: ttt.Validator{},
	})
	if err != nil {
		fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		fatal(err)
	}
	stdin := bufio.NewScanner(os.Stdin)
	// The watcher shows the final position when no human is seated to
	// render it. It taps the X agent's observations from outside the
	// game: spectating is presentation, not a rule.
	watcher := &watchedAgent{inner: makeAgent(*xKind, stdin)}
	if err := s.Admit(ttt.SlotX, watcher); err != nil {
		fatal(err)
	}
	if err := s.Admit(ttt.SlotO, makeAgent(*yKind, stdin)); err != nil {
		fatal(err)
	}
	if err := s.Run(context.Background()); err != nil {
		fatal(err)
	}
	if *xKind == "bot" && *yKind == "bot" {
		fmt.Println(render(watcher.last.Board))
		fmt.Println("X:", watcher.result.Signal.Terminal)
	}
}

// watchedAgent forwards everything to the wrapped agent while retaining
// the last observation and result for the spectator printout.
type watchedAgent struct {
	inner  session.Agent[ttt.Observation, ttt.Move]
	last   ttt.Observation
	result session.Result
}

func (w *watchedAgent) Guest(slot session.SlotID) { w.inner.Guest(slot) }

func (w *watchedAgent) Observe(obs ttt.Observation) {
	w.last = obs
	w.inner.Observe(obs)
}

func (w *watchedAgent) Decide(ctx context.Context) (ttt.Move, bool) { return w.inner.Decide(ctx) }

func (w *watchedAgent) Ended(r session.Result) {
	w.result = r
	w.inner.Ended(r)
}

func makeAgent(kind string, stdin *bufio.Scanner) session.Agent[ttt.Observation, ttt.Move] {
	switch kind {
	case "human":
		return &consoleAgent{stdin: stdin}
	case "bot":
		return &ttt.Bot{}
	default:
		fatal(fmt.Errorf("unknown controller kind %q (want human or bot)", kind))
		return nil
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ttt:", err)
	os.Exit(1)
}

// consoleAgent is actor:human-agent for a terminal: it renders the
// observation and turns typed cell numbers into moves. It sees exactly
// what the bot sees — the Observation, never the world state.
type consoleAgent struct {
	stdin *bufio.Scanner
	last  ttt.Observation
	slot  session.SlotID
}

func (c *consoleAgent) Guest(slot session.SlotID) {
	c.slot = slot
	fmt.Printf("You are %s\n", markName(slot))
}

func (c *consoleAgent) Observe(obs ttt.Observation) {
	c.last = obs
	// Only narrate when something is worth showing: our turn, or the end.
	if obs.NextTurn == c.slot || obs.NextTurn == 0 {
		fmt.Println(render(obs.Board))
	}
}

func (c *consoleAgent) Decide(ctx context.Context) (ttt.Move, bool) {
	for {
		fmt.Printf("%s to move — cell (0-8): ", markName(c.slot))
		if !c.stdin.Scan() {
			return ttt.Move{}, false // EOF: no action, session drains
		}
		n, err := strconv.Atoi(strings.TrimSpace(c.stdin.Text()))
		if err != nil || n < 0 || n > 8 {
			fmt.Println("enter a number 0-8")
			continue
		}
		if c.last.Board[n] != ttt.Empty {
			fmt.Println("that cell is taken")
			continue
		}
		return ttt.Move{Cell: uint8(n)}, true
	}
}

func (c *consoleAgent) Ended(r session.Result) {
	switch r.Signal.Terminal {
	case session.Win:
		fmt.Printf("%s wins!\n", markName(c.slot))
	case session.Lose:
		fmt.Printf("%s loses.\n", markName(c.slot))
	case session.Draw:
		fmt.Println("Draw.")
	default:
		fmt.Println("Game abandoned.")
	}
}

// markName renders SlotO as "Y": the board indexes empty cells 0-8, and
// an O mark is too easy to misread as the digit 0 beside them.
func markName(slot session.SlotID) string {
	if slot == ttt.SlotX {
		return "X"
	}
	return "Y"
}

func render(b ttt.Board) string {
	sym := func(i int) string {
		switch b[i] {
		case ttt.MarkX:
			return "X"
		case ttt.MarkO:
			return "Y"
		default:
			return strconv.Itoa(i)
		}
	}
	var sb strings.Builder
	for row := range 3 {
		sb.WriteString(" " + sym(row*3) + " | " + sym(row*3+1) + " | " + sym(row*3+2) + "\n")
		if row < 2 {
			sb.WriteString("---+---+---\n")
		}
	}
	return sb.String()
}
