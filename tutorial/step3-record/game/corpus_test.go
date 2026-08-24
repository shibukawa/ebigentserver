package game_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/analysis"
	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/tutorial/step3-record/game"
	"github.com/shibukawa/ebigentserver/tutorial/step3-record/msg"
)

// TestYourOwnPlayIsWhatGetsRecorded is step 3 without the window.
//
// One seat is a person — the same detached seat a click fills, submitting
// through run.Controls exactly as Intake does — and the other is the
// stand-in. What the test then reads out of decisions.jsonl is not the
// board history. It is, for each move the person made, the sight they had
// in front of them when they made it, which is the thing a policy can be
// recovered from and the thing an ordinary game loop never keeps.
func TestYourOwnPlayIsWhatGetsRecorded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	root := t.TempDir()
	roster, err := run.NewRoster[msg.TTTWorld, msg.Move, game.Sight](game.Options(), game.Slots())
	if err != nil {
		t.Fatal(err)
	}
	// A click in the lobby, and then the press that says "stop waiting".
	if _, err := roster.SitLocal("player"); err != nil {
		t.Fatal(err)
	}
	if err := roster.FillBots(game.Binding().NewAgent); err != nil {
		t.Fatal(err)
	}

	rec, err := run.OpenRecording[msg.TTTWorld, msg.Move, game.Sight](run.RecordOptions{
		Root:              root,
		EpisodeID:         "tictactoe-0000",
		ProtocolVersion:   game.Protocol,
		EvaluationVersion: game.Evaluation,
		// The seat labels come from the roster, which is the only place
		// that knows a person took one. Nothing in the rules could have
		// supplied this column.
		AgentKinds: roster.AgentKinds(),
	})
	if err != nil {
		t.Fatal(err)
	}

	human := &player{}
	cfg := game.Config("tictactoe-0000", 1)
	cfg.Recorder = rec.Recorder()
	cfg.Broadcast = human.apply

	match, err := roster.Finalize(cfg)
	if err != nil {
		rec.Close()
		t.Fatal(err)
	}
	match.Start(ctx, session.Paced)

	frames, stopFrames := context.WithCancel(ctx)
	defer stopFrames()
	go pump(frames, human, match)

	select {
	case <-match.Done():
	case <-ctx.Done():
		t.Fatal("the match never finished")
	}
	stopFrames()
	if err := match.Err(); err != nil {
		t.Fatalf("match: %v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("recording: %v", err)
	}

	header, rows := readDecisions(t, rec.Dir)
	if header.ProtocolVersion != game.Protocol {
		t.Fatalf("header protocol %q, want %q", header.ProtocolVersion, game.Protocol)
	}

	var mine, closing int
	for _, row := range rows {
		if session.SlotID(row.Slot) != game.SlotX {
			continue
		}
		if row.AgentKind != "human" {
			t.Fatalf("tick %d: agent_kind %q, want human", row.Tick, row.AgentKind)
		}
		var sight game.Sight
		if err := json.Unmarshal(row.Sight, &sight); err != nil {
			t.Fatalf("tick %d: sight: %v", row.Tick, err)
		}

		if len(row.Action) == 0 {
			// A row with a null action is a sight nobody acted on.
			// There is one per seat, at the end: the session shows
			// everybody the finished position before it closes, and
			// that row is where how it ended is written down.
			if !sight.Over {
				t.Fatalf("tick %d: a row with no action on an unfinished board", row.Tick)
			}
			closing++
			continue
		}
		mine++

		var move msg.Move
		if err := json.Unmarshal(row.Action, &move); err != nil {
			t.Fatalf("tick %d: action: %v", row.Tick, err)
		}

		// The claim step 4 rests on: the sight beside a move is the
		// position that move answered, complete enough to judge it by.
		// If it were the position afterwards, or the board with no
		// legality on it, nothing downstream could tell a considered
		// move from the only one available.
		if len(sight.Cells) != 9 {
			t.Fatalf("tick %d: sight carries %d cells", row.Tick, len(sight.Cells))
		}
		if !slices.Contains(sight.Legal, int(move.Cell)) {
			t.Fatalf("tick %d: played %d, which the recorded sight lists as illegal (legal: %v)",
				row.Tick, move.Cell, sight.Legal)
		}
		if sight.Cells[move.Cell] != game.Empty {
			t.Fatalf("tick %d: the recorded sight already holds a mark on %d — that is the board after the move, not before it",
				row.Tick, move.Cell)
		}
	}
	if mine == 0 {
		t.Fatal("the person's seat produced no moves at all")
	}
	if closing != 1 {
		t.Fatalf("%d terminal rows for the person's seat, want exactly 1", closing)
	}
	if terminal := rows[len(rows)-1].Evaluation.Terminal; terminal == "" {
		t.Fatal("the last row carries no terminal outcome")
	}

	// Both seats were recorded the same way. Nothing but the label
	// separates the person's rows from the bot's, which is what makes
	// one corpus out of two kinds of controller.
	if kinds := labels(rows); !slices.Equal(kinds, []string{"bot", "human"}) {
		t.Fatalf("agent kinds in the log = %v, want both human and bot", kinds)
	}
	t.Logf("%d rows, %d of them moves the person made", len(rows), mine)
}

// TestPlayingWithNobodyWatchingProducesACorpus runs the same rules with
// no window, no person, and no clock: every seat is filled from the same
// Binding.NewAgent the lobby's press uses, and the run finishes in the
// time it takes to open the files.
//
// It is here because it is the one honest way to get a corpus of any
// size, and because it is the same call an automated collector would
// make. Nothing about it is a different mode — Serve is the match loop
// with the gathering step answered in advance.
func TestPlayingWithNobodyWatchingProducesACorpus(t *testing.T) {
	root := recordHeadless(t, 4)

	corpus, err := analysis.LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Episodes) != 4 {
		t.Fatalf("%d episodes recorded, want 4", len(corpus.Episodes))
	}
	report := analysis.Compute(corpus)
	if report.ActionRows == 0 {
		t.Fatal("the corpus holds no decisions")
	}
	if len(report.ResultsBySlot) != 2 {
		t.Fatalf("outcomes cover %d seats, want 2", len(report.ResultsBySlot))
	}
	// A stand-in that answered on turns it could not act would show up
	// here, one rejection per tick, long before anybody read the file.
	if len(report.Rejections) != 0 {
		t.Fatalf("the log holds validator rejections: %v", report.Rejections)
	}
	t.Logf("%d episodes, %d decisions, results %v",
		report.Episodes, report.ActionRows, report.ResultsBySlot)
}

// TestTwoCopiesOfTheBotRecordTheSameGameEveryTime is the limit of the
// paragraph above, and it is deliberately a passing test rather than a
// caveat in a comment.
//
// The stand-in is deterministic and so is the game, so a headless run of
// four matches is one match written down four times. Volume is not
// variety: a corpus is worth what the play that produced it was worth,
// which is why step 3 is about recording a person and why anything
// automated later has to introduce difference on purpose — different
// openings, different opponents, different policies playing each other.
func TestTwoCopiesOfTheBotRecordTheSameGameEveryTime(t *testing.T) {
	root := recordHeadless(t, 4)

	dirs, err := filepath.Glob(filepath.Join(root, "*"))
	if err != nil || len(dirs) < 2 {
		t.Fatalf("glob %v: %d episodes", err, len(dirs))
	}
	slices.Sort(dirs)

	_, first := readDecisions(t, dirs[0])
	for _, dir := range dirs[1:] {
		_, other := readDecisions(t, dir)
		if len(other) != len(first) {
			t.Fatalf("%s has %d rows, %s has %d — the run stopped being deterministic",
				dir, len(other), dirs[0], len(first))
		}
		for i := range first {
			if !slices.Equal(first[i].Sight, other[i].Sight) || !slices.Equal(first[i].Action, other[i].Action) {
				t.Fatalf("%s row %d differs from %s", dir, i, dirs[0])
			}
		}
	}
	t.Logf("%d episodes, %d distinct games", len(dirs), 1)
}

// recordHeadless plays matches with nobody watching and returns the
// corpus root.
func recordHeadless(t *testing.T, matches int) string {
	t.Helper()
	root := t.TempDir()
	err := run.Serve(context.Background(), game.Options(), game.Binding(), run.ServeOptions{
		Matches: matches,
		Seed:    1,
		Time:    session.Unlimited,
		Record:  run.RecordOptions{Root: root},
	})
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	return root
}

// readDecisions reads one episode's decisions stream back: the header
// row, then every decision after it.
func readDecisions(t *testing.T, dir string) (episode.Header, []episode.Decision) {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var header episode.Header
	var rows []episode.Decision
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if header.Stream == "" {
			if err := json.Unmarshal(line, &header); err != nil {
				t.Fatalf("%s: header: %v", dir, err)
			}
			continue
		}
		var row episode.Decision
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("%s: row: %v", dir, err)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if header.Stream != "decisions" {
		t.Fatalf("%s: first row is %q, not the stream header", dir, header.Stream)
	}
	return header, rows
}

// labels is the sorted set of agent kinds a decisions stream mentions.
func labels(rows []episode.Decision) []string {
	var out []string
	for _, row := range rows {
		if !slices.Contains(out, row.AgentKind) {
			out = append(out, row.AgentKind)
		}
	}
	slices.Sort(out)
	return out
}
