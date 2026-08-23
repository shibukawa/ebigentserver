package game_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/analysis"
	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/examples/solo/game"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
)

// TestSoloProducesATrainableCorpus is the reason this sample is a solo
// game on a session framework rather than a loop with an enemy update
// function.
//
// It plays matches nobody watched and then checks the corpus for what
// flow:behavior-tree-synthesis will need from it: a decision per enemy
// per tick, each carrying the sight it was made from, labelled by
// who decided and separable by episode outcome. Nothing in the game
// arranges any of that — it falls out of the enemies being seats.
func TestSoloProducesATrainableCorpus(t *testing.T) {
	root := t.TempDir()
	if err := run.Serve(context.Background(), game.Options(), game.Binding(), run.ServeOptions{
		Matches: 6,
		Seed:    1,
		Time:    session.Unlimited,
		Record:  run.RecordOptions{Root: root, Mode: episode.ReplayComplete},
	}); err != nil {
		t.Fatal(err)
	}

	corpus, err := analysis.LoadCorpus(root)
	if err != nil {
		t.Fatalf("the corpus the tools read is unreadable: %v", err)
	}
	if len(corpus.Episodes) != 6 {
		t.Fatalf("corpus holds %d episodes, wanted 6", len(corpus.Episodes))
	}

	outcomes := map[string]int{}
	for _, ep := range corpus.Episodes {
		if ep.Header.ProtocolVersion != game.Protocol {
			t.Errorf("%s: protocol %q, wanted %q — a corpus that mixes schemas cannot be distilled",
				ep.Dir, ep.Header.ProtocolVersion, game.Protocol)
		}
		if ep.Header.Mode != episode.ReplayComplete {
			t.Errorf("%s: mode %q, so the episode cannot be replayed and checked", ep.Dir, ep.Header.Mode)
		}

		// Every enemy has to appear as a decider. An enemy whose moves
		// were applied without being recorded is exactly the case this
		// whole arrangement exists to prevent.
		decided := map[uint16]int{}
		for _, row := range ep.Decisions {
			if row.HasAction {
				decided[row.Slot]++
			}
		}
		for _, slot := range []session.SlotID{game.Enemy1, game.Enemy2} {
			if decided[uint16(slot)] == 0 {
				t.Errorf("%s: enemy slot %d made no recorded decision", ep.Dir, slot)
			}
		}
		if decided[uint16(game.Player)] == 0 {
			t.Errorf("%s: the quarry made no recorded decision", ep.Dir)
		}
		for _, row := range ep.Decisions {
			if row.AgentKind != "bot" {
				t.Errorf("%s: slot %d recorded as %q; an unattended run seats bots",
					ep.Dir, row.Slot, row.AgentKind)
				break
			}
		}

		for _, o := range ep.Outcomes {
			if session.SlotID(o.Slot) == game.Player {
				outcomes[o.Result]++
			}
		}
	}

	// Both classes have to be present or a distilled enemy would have
	// nothing to separate a good pursuit from a bad one.
	if outcomes["win"] == 0 || outcomes["lose"] == 0 {
		t.Errorf("six matches produced outcomes %v; a trainable corpus needs both", outcomes)
	}
}

// TestRecordedDecisionCarriesItsSight checks the content of one
// decision row rather than the shape of the corpus. The sight as
// delivered is the record (data:decision-record): a distilled predicate
// is written against these fields, so if the quarry's position were
// missing here, no enemy could be distilled from this log at all.
func TestRecordedDecisionCarriesItsSight(t *testing.T) {
	root := t.TempDir()
	if err := run.Serve(context.Background(), game.Options(), game.Binding(), run.ServeOptions{
		Matches: 1,
		Seed:    1,
		Time:    session.Unlimited,
		Record:  run.RecordOptions{Root: root, Mode: episode.ReplayComplete},
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read corpus: %v (%d entries)", err, len(entries))
	}
	f, err := os.Open(filepath.Join(root, entries[0].Name(), "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	type row struct {
		Slot   uint16       `json:"slot"`
		Sight  game.Sight   `json:"sight"`
		Action *game.Action `json:"action"`
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	found := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.Contains(line, `"stream"`) {
			continue // the header line
		}
		var r row
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("decision row is not decodable as this game's types: %v", err)
		}
		if session.SlotID(r.Slot) != game.Enemy1 || r.Action == nil {
			continue
		}
		if r.Sight.You != game.Enemy1 {
			t.Fatalf("row for slot %d carries an sight belonging to slot %d",
				r.Slot, r.Sight.You)
		}
		if r.Sight.Quarry.X == 0 && r.Sight.Quarry.Y == 0 {
			t.Fatal("the recorded sight has no quarry position, so nothing could be learned from it")
		}
		if len(r.Sight.Others) != game.Seats-1 {
			t.Fatalf("the recorded sight holds %d other actors, wanted %d",
				len(r.Sight.Others), game.Seats-1)
		}
		found = true
		break
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("no enemy decision row was found in the episode")
	}
}
