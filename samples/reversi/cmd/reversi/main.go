// Command reversi plays sample:reversi in the terminal. Controllers per
// slot are launch flags (decision:no-ai-game-mode); "human", "greedy",
// and "first" all sit behind the same session.Agent interface. With
// -record, the match is written as a replay_complete data:episode-log
// under the given directory.
//
//	reversi                       # human plays black against the greedy bot
//	reversi -black=first -white=greedy
//	reversi -black=greedy -white=greedy -record=./episodes
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/reversi/reversi"
	"github.com/shibukawa/ebigentserver/session"
)

func main() {
	blackKind := flag.String("black", "human", "controller for black: human, greedy, or first")
	whiteKind := flag.String("white", "greedy", "controller for white: human, greedy, or first")
	record := flag.String("record", "", "directory to write the episode log into")
	flag.Parse()

	cfg := session.Config[reversi.State, reversi.Move, reversi.Sight]{
		ID:        "reversi-cli",
		Slots:     reversi.Slots(),
		RuleSet:   reversi.RuleSet{},
		Validator: reversi.Validator{},
		Canonical: reversi.Canonical,
	}

	var recorder *episode.Writer[reversi.State, reversi.Move, reversi.Sight]
	if *record != "" {
		files, closeAll, err := openStreams(*record)
		if err != nil {
			fatal(err)
		}
		defer closeAll()
		recorder = episode.NewWriter[reversi.State, reversi.Move, reversi.Sight](
			files, episode.ReplayComplete,
			episode.Meta{AgentKinds: map[session.SlotID]string{
				reversi.SlotBlack: *blackKind,
				reversi.SlotWhite: *whiteKind,
			}},
		)
		cfg.Recorder = recorder
	}

	s, err := session.New(cfg)
	if err != nil {
		fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		fatal(err)
	}
	stdin := bufio.NewScanner(os.Stdin)
	watcher := &watchedAgent{inner: makeAgent(*blackKind, stdin)}
	if err := s.Admit(reversi.SlotBlack, watcher); err != nil {
		fatal(err)
	}
	if err := s.Admit(reversi.SlotWhite, makeAgent(*whiteKind, stdin)); err != nil {
		fatal(err)
	}
	if err := s.Run(context.Background()); err != nil {
		fatal(err)
	}
	if *blackKind != "human" && *whiteKind != "human" {
		fmt.Println(render(watcher.last.Board))
		fmt.Printf("black: %v (%d discs) in %d moves\n",
			watcher.result.Signal.Terminal, watcher.last.Signal.Score, s.Tick())
	}
	if recorder != nil {
		if err := recorder.Err(); err != nil {
			fatal(fmt.Errorf("recording: %w", err))
		}
		fmt.Println("episode recorded to", *record)
	}
}

func openStreams(dir string) (episode.Streams, func(), error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return episode.Streams{}, nil, err
	}
	var files []*os.File
	open := func(name string) (*os.File, error) {
		f, err := os.Create(filepath.Join(dir, name))
		if err == nil {
			files = append(files, f)
		}
		return f, err
	}
	var s episode.Streams
	var err error
	if s.Decisions, err = open("decisions.jsonl"); err == nil {
		if s.Events, err = open("events.jsonl"); err == nil {
			if s.Outcomes, err = open("outcomes.jsonl"); err == nil {
				s.World, err = open("world.jsonl")
			}
		}
	}
	closeAll := func() {
		for _, f := range files {
			f.Close()
		}
	}
	if err != nil {
		closeAll()
		return episode.Streams{}, nil, err
	}
	return s, closeAll, nil
}

func makeAgent(kind string, stdin *bufio.Scanner) session.Agent[reversi.Sight, reversi.Move] {
	switch kind {
	case "human":
		return &consoleAgent{stdin: stdin}
	case "greedy":
		return &reversi.GreedyBot{}
	case "first":
		return &reversi.FirstBot{}
	default:
		fatal(fmt.Errorf("unknown controller kind %q (want human, greedy, or first)", kind))
		return nil
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "reversi:", err)
	os.Exit(1)
}

type watchedAgent struct {
	inner  session.Agent[reversi.Sight, reversi.Move]
	last   reversi.Sight
	result session.Result
}

func (w *watchedAgent) Joined(slot session.SlotID) { w.inner.Joined(slot) }

func (w *watchedAgent) Observe(obs reversi.Sight) {
	w.last = obs
	w.inner.Observe(obs)
}

func (w *watchedAgent) Decide(ctx context.Context) (reversi.Move, bool) { return w.inner.Decide(ctx) }

func (w *watchedAgent) Ended(r session.Result) {
	w.result = r
	w.inner.Ended(r)
}

// consoleAgent is the human seat: it renders the sight and parses
// coordinates like "d3". It chooses only from Sight.Legal, exactly
// like the bots — no private rule engine anywhere.
type consoleAgent struct {
	stdin *bufio.Scanner
	last  reversi.Sight
	slot  session.SlotID
}

func (c *consoleAgent) Joined(slot session.SlotID) {
	c.slot = slot
	fmt.Printf("You are %s\n", discName(slot))
}

func (c *consoleAgent) Observe(obs reversi.Sight) {
	c.last = obs
	if obs.NextTurn == c.slot || obs.NextTurn == 0 {
		fmt.Println(render(obs.Board))
	}
}

func (c *consoleAgent) Decide(context.Context) (reversi.Move, bool) {
	legal := c.last.Legal
	if len(legal) == 1 && legal[0].Move.Pass {
		fmt.Println("no legal move — you pass")
		return reversi.Move{Pass: true}, true
	}
	var opts []string
	for _, lm := range legal {
		opts = append(opts, cellName(lm.Move.Cell))
	}
	for {
		fmt.Printf("%s to move [%s]: ", discName(c.slot), strings.Join(opts, " "))
		if !c.stdin.Scan() {
			return reversi.Move{}, false
		}
		cell, ok := parseCell(strings.TrimSpace(c.stdin.Text()))
		if !ok {
			fmt.Println("enter a coordinate like d3")
			continue
		}
		for _, lm := range legal {
			if lm.Move.Cell == cell {
				return lm.Move, true
			}
		}
		fmt.Println("not a legal move")
	}
}

func (c *consoleAgent) Ended(r session.Result) {
	switch r.Signal.Terminal {
	case session.Win:
		fmt.Printf("%s wins with %d discs!\n", discName(c.slot), r.Signal.Score)
	case session.Lose:
		fmt.Printf("%s loses with %d discs.\n", discName(c.slot), r.Signal.Score)
	case session.Draw:
		fmt.Println("Draw.")
	default:
		fmt.Println("Game abandoned.")
	}
}

func discName(slot session.SlotID) string {
	if slot == reversi.SlotBlack {
		return "black (●)"
	}
	return "white (○)"
}

func cellName(cell uint8) string {
	return string(rune('a'+cell%8)) + strconv.Itoa(int(cell/8)+1)
}

func parseCell(s string) (uint8, bool) {
	if len(s) != 2 || s[0] < 'a' || s[0] > 'h' || s[1] < '1' || s[1] > '8' {
		return 0, false
	}
	return (s[1]-'1')*8 + (s[0] - 'a'), true
}

func render(b reversi.Board) string {
	var sb strings.Builder
	sb.WriteString("  a b c d e f g h\n")
	for row := range 8 {
		sb.WriteString(strconv.Itoa(row + 1))
		for col := range 8 {
			sb.WriteByte(' ')
			switch b[row*8+col] {
			case reversi.Black:
				sb.WriteString("●")
			case reversi.White:
				sb.WriteString("○")
			default:
				sb.WriteString("·")
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
