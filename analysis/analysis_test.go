package analysis

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/tictactoe/ttt"
	"github.com/shibukawa/ebigentserver/session"
)

// epSpec describes one generated tic-tac-toe episode.
type epSpec struct {
	id         string
	protocol   string
	mode       episode.Mode
	oKind      string
	oMoves     []ttt.Move // nil: O is a second first-empty Bot
	canonical  bool       // enable checkpoints
	withEvents bool
	withWorld  bool
}

// playEpisode runs one bot-vs-opponent match through the public session
// API exactly as samples/tictactoe/distill does, writing the four JSONL
// streams as files in their own episode directory.
func playEpisode(t *testing.T, root string, seed uint64, spec epSpec) {
	t.Helper()
	dir := filepath.Join(root, spec.id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	streams := episode.Streams{
		Decisions: open("decisions.jsonl"),
		Outcomes:  open("outcomes.jsonl"),
	}
	if spec.withEvents {
		streams.Events = open("events.jsonl")
	}
	if spec.withWorld {
		streams.World = open("world.jsonl")
	}
	w := episode.NewWriter[ttt.State, ttt.Move, ttt.Sight](
		streams, spec.mode,
		episode.Meta{
			EpisodeID:       spec.id,
			ProtocolVersion: spec.protocol,
			AgentKinds:      map[session.SlotID]string{ttt.SlotX: "bot", ttt.SlotO: spec.oKind},
		},
	)
	cfg := session.Config[ttt.State, ttt.Move, ttt.Sight]{
		ID: spec.id, Slots: ttt.Slots(),
		RuleSet: ttt.RuleSet{}, Validator: ttt.Validator{},
		Recorder: w, Seed: seed,
		Clock: func() int64 { return 0 },
	}
	if spec.canonical {
		cfg.Canonical = func(s *ttt.State) []byte {
			b, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
	}
	s, err := session.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	if err := s.Admit(ttt.SlotX, &ttt.Bot{}); err != nil {
		t.Fatal(err)
	}
	var opponent session.Agent[ttt.Sight, ttt.Move] = &ttt.Bot{}
	if spec.oMoves != nil {
		opponent = &ttt.Script{Moves: spec.oMoves}
	}
	if err := s.Admit(ttt.SlotO, opponent); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := w.Err(); err != nil {
		t.Fatalf("episode %s: writer error: %v", spec.id, err)
	}
}

// firstEmpty are the cells a first-empty O plays against the first-empty
// X bot (X takes 0,2,4,6 and wins on move 7).
var firstEmpty = []ttt.Move{{Cell: 1}, {Cell: 3}, {Cell: 5}}

// buildCorpus plays 8 episodes: 5 plain replay_complete bot-vs-script,
// one with a leading illegal move (a validator rejection), and two
// analysis_sampled bot-vs-bot under a second protocol version.
func buildCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specs := []epSpec{
		{id: "ep-000", protocol: "v1", mode: episode.ReplayComplete, oKind: "scripted", oMoves: firstEmpty, canonical: true, withEvents: true, withWorld: true},
		{id: "ep-001", protocol: "v1", mode: episode.ReplayComplete, oKind: "scripted", oMoves: firstEmpty, canonical: true, withEvents: true, withWorld: true},
		{id: "ep-002", protocol: "v1", mode: episode.ReplayComplete, oKind: "scripted", oMoves: firstEmpty, canonical: true, withEvents: true, withWorld: true},
		{id: "ep-003", protocol: "v1", mode: episode.ReplayComplete, oKind: "scripted", oMoves: firstEmpty, canonical: true, withEvents: true, withWorld: true},
		{id: "ep-004", protocol: "v1", mode: episode.ReplayComplete, oKind: "scripted", oMoves: firstEmpty, canonical: true, withEvents: true, withWorld: true},
		// Cell 0 is occupied when O first moves: one rejection, then
		// the retry falls through to the legal script tail.
		{id: "ep-005", protocol: "v1", mode: episode.ReplayComplete, oKind: "scripted",
			oMoves: append([]ttt.Move{{Cell: 0}}, firstEmpty...), canonical: true, withEvents: true, withWorld: true},
		// analysis_sampled: no world stream; ep-007 also drops events
		// to exercise the missing-events tolerance.
		{id: "ep-006", protocol: "v2", mode: episode.AnalysisSampled, oKind: "bot", withEvents: true},
		{id: "ep-007", protocol: "v2", mode: episode.AnalysisSampled, oKind: "bot"},
	}
	for i, spec := range specs {
		playEpisode(t, root, uint64(i)*2654435761+1, spec)
	}
	return root
}

func TestComputeOverGeneratedCorpus(t *testing.T) {
	root := buildCorpus(t)
	c, err := LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(c.Episodes); got != 8 {
		t.Fatalf("episodes = %d, want 8", got)
	}
	totalOutcomes := 0
	for _, ep := range c.Episodes {
		totalOutcomes += len(ep.Outcomes)
		for _, o := range ep.Outcomes {
			if o.DurationTicks != 7 {
				t.Errorf("%s slot %d: duration %d, want 7 (bot-vs-first-empty ttt is deterministic)", ep.Dir, o.Slot, o.DurationTicks)
			}
		}
	}
	if totalOutcomes != 16 {
		t.Fatalf("total outcomes = %d, want episodes*2 = 16", totalOutcomes)
	}

	r := Compute(c)
	if r.Episodes != 8 {
		t.Fatalf("report episodes = %d, want 8", r.Episodes)
	}

	// Header/protocol/mode grouping.
	if r.ByProtocol["v1"] != 6 || r.ByProtocol["v2"] != 2 || len(r.ByProtocol) != 2 {
		t.Errorf("ByProtocol = %v, want v1:6 v2:2", r.ByProtocol)
	}
	if r.ByMode["replay_complete"] != 6 || r.ByMode["analysis_sampled"] != 2 || len(r.ByMode) != 2 {
		t.Errorf("ByMode = %v, want replay_complete:6 analysis_sampled:2", r.ByMode)
	}

	// X (slot 1) always wins in 7 ticks against a first-empty O.
	if got := r.ResultsBySlot[1]["win"]; got != 8 {
		t.Errorf("slot 1 wins = %d, want 8", got)
	}
	if got := r.ResultsBySlot[2]["lose"]; got != 8 {
		t.Errorf("slot 2 losses = %d, want 8", got)
	}
	// By agent kind: "bot" holds X in all 8 plus O in the two
	// bot-vs-bot episodes; "scripted" holds O in the other six.
	if got := r.ResultsByAgentKind["bot"]; got["win"] != 8 || got["lose"] != 2 {
		t.Errorf(`kind "bot" = %v, want win:8 lose:2`, got)
	}
	if got := r.ResultsByAgentKind["scripted"]; got["lose"] != 6 || len(got) != 1 {
		t.Errorf(`kind "scripted" = %v, want lose:6 only`, got)
	}

	// Duration stats are exact: every episode lasts 7 ticks.
	if r.Duration.MinTicks != 7 || r.Duration.MaxTicks != 7 || r.Duration.MeanTicks != 7 {
		t.Errorf("duration = min %d mean %v max %d, want all 7",
			r.Duration.MinTicks, r.Duration.MeanTicks, r.Duration.MaxTicks)
	}
	if r.Duration.Histogram[0].Label != "0-9" || r.Duration.Histogram[0].Count != 8 {
		t.Errorf("histogram[0] = %+v, want 0-9: 8", r.Duration.Histogram[0])
	}
	for _, b := range r.Duration.Histogram[1:] {
		if b.Count != 0 {
			t.Errorf("histogram bucket %s = %d, want 0", b.Label, b.Count)
		}
	}

	// Per tick both slots get a row (one Decided, one Observed), 7
	// ticks, plus the final sight delivered to both at drain:
	// 16 rows per episode, 7 of them with actions.
	if r.DecisionRows != 128 || r.ActionRows != 56 || r.SightRows != 72 {
		t.Errorf("decision rows = %d (action %d, obs %d), want 128/56/72",
			r.DecisionRows, r.ActionRows, r.SightRows)
	}
	if r.DecisionsPerEpisodeMean != 16 {
		t.Errorf("decisions per episode mean = %v, want 16", r.DecisionsPerEpisodeMean)
	}

	// Exactly one rejected move across the corpus (ep-005's leading
	// play on the occupied cell 0).
	if len(r.Rejections) != 1 || r.Rejections["ttt: cell 0 is occupied"] != 1 {
		t.Errorf("rejections = %v, want 1x cell-0-occupied", r.Rejections)
	}

	// Checkpoints: 6 replay_complete episodes with Canonical set,
	// CheckpointEvery defaulting to 1, 7 ticks each.
	if r.Checkpoints != 42 {
		t.Errorf("checkpoints = %d, want 42", r.Checkpoints)
	}

	// Stream byte totals track real files.
	for _, stream := range []string{"decisions", "events", "outcomes", "world"} {
		if r.StreamBytes[stream] <= 0 {
			t.Errorf("stream bytes %s = %d, want > 0", stream, r.StreamBytes[stream])
		}
	}
}

func TestWriteTextIsReadableAndDeterministic(t *testing.T) {
	root := buildCorpus(t)
	c, err := LoadCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	r := Compute(c)
	var a, b strings.Builder
	r.WriteText(&a)
	r.WriteText(&b)
	if a.String() != b.String() {
		t.Fatal("WriteText output is not deterministic")
	}
	for _, want := range []string{
		"episodes: 8",
		"v1: 6",
		"v2: 2",
		"replay_complete: 6",
		"analysis_sampled: 2",
		"slot 1: win=8",
		"slot 2: lose=8",
		"bot: win=8 lose=2",
		"scripted: lose=6",
		"duration ticks: min=7 mean=7.00 max=7",
		"0-9: 8",
		"decision rows: 128 (mean 16.00 per episode)",
		"with action: 56",
		"sight-only: 72",
		"checkpoints: 42",
		"1  ttt: cell 0 is occupied",
		"decisions: ",
		"total: ",
	} {
		if !strings.Contains(a.String(), want) {
			t.Errorf("WriteText output missing %q\n---\n%s", want, a.String())
		}
	}
}

func TestWriteDuckDBSQL(t *testing.T) {
	var a, b strings.Builder
	if err := WriteDuckDBSQL(&a, "/data/corpus"); err != nil {
		t.Fatal(err)
	}
	if err := WriteDuckDBSQL(&b, "/data/corpus"); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatal("WriteDuckDBSQL output is not stable")
	}
	for _, want := range []string{
		"duckdb -init",
		"CREATE OR REPLACE VIEW decisions AS",
		"CREATE OR REPLACE VIEW events AS",
		"CREATE OR REPLACE VIEW outcomes AS",
		"CREATE OR REPLACE VIEW world AS",
		"read_json('/data/corpus/*/decisions.jsonl', format='newline_delimited', filename=true,",
		"WHERE stream IS NULL",
		"win_rate",
		"duration_ticks",
		"kind = 'rejected'",
		"observation_rows",
	} {
		if !strings.Contains(a.String(), want) {
			t.Errorf("SQL output missing %q", want)
		}
	}
	// Single quotes in the corpus path must not break the script.
	var q strings.Builder
	if err := WriteDuckDBSQL(&q, "/data/it's here"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q.String(), "it''s here") {
		t.Error("single quote in root not escaped")
	}
}
