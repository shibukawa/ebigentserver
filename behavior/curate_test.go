package behavior

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEpisode puts one episode directory into a corpus root. Rows are
// raw JSONL lines so a test states exactly what the recorder wrote.
func writeEpisode(t *testing.T, root, name string, rows ...string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := append([]string{`{"stream":"decisions","schema_version":1,"episode_id":"` + name + `"}`}, rows...)
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "decisions.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func row(tick int, slot int, kind, sight, action string) string {
	return `{"tick":` + itoa(tick) + `,"slot":` + itoa(slot) + `,"agent_kind":"` + kind +
		`","sight":` + sight + `,"action":` + action + `}`
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestCurateFiltersAggregatesAndCaps(t *testing.T) {
	root, out := t.TempDir(), filepath.Join(t.TempDir(), "curated")
	// Episode a: the same situation three times by the human, once by
	// the bot with a different action (a conflict once the bot rows are
	// kept, a duplicate pile once they are not), plus one sight-only row.
	writeEpisode(t, root, "a",
		row(1, 1, "human", `{"board":"open"}`, `{"cell":4}`),
		row(2, 2, "coin", `{"board":"open"}`, `{"cell":8}`),
		row(3, 1, "human", `{"board":"open"}`, `{"cell":4}`),
		`{"tick":4,"slot":1,"agent_kind":"human","sight":{"board":"open"},"action":null}`,
		row(5, 1, "human", `{"board":"open"}`, `{"cell":4}`),
	)
	writeEpisode(t, root, "b",
		row(1, 1, "human", `{"board":"rare"}`, `{"cell":0}`),
	)

	rep, err := Curate(root, out, CurateOptions{
		Filter: RowFilter{AgentKind: "human"},
		Cap:    2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Read != 6 || rep.SightOnly != 1 || rep.FilteredOut != 1 {
		t.Fatalf("read %d, sight-only %d, filtered %d; want 6, 1, 1", rep.Read, rep.SightOnly, rep.FilteredOut)
	}
	if rep.TrainRows != 4 || rep.TrainKept != 3 || rep.TrainDropped != 1 {
		t.Fatalf("train rows %d kept %d dropped %d; want 4, 3, 1", rep.TrainRows, rep.TrainKept, rep.TrainDropped)
	}
	if rep.Situations != 2 {
		t.Fatalf("situations %d, want 2", rep.Situations)
	}
	// The bot's diverging action was filtered out, so nothing conflicts.
	if len(rep.Conflicts) != 0 {
		t.Fatalf("conflicts %v, want none", rep.Conflicts)
	}
	if d := rep.TopDuplicates[0]; d.Rows != 3 || d.Kept != 2 {
		t.Fatalf("top duplicate rows %d kept %d; want 3, 2", d.Rows, d.Kept)
	}

	// The curated corpus must round-trip through the ordinary reader.
	body, err := os.ReadFile(filepath.Join(out, "train", "a", "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(body), "\n"); lines != 3 { // header + 2 kept rows
		t.Fatalf("episode a kept %d lines, want 3", lines)
	}
	if _, err := os.Stat(filepath.Join(out, "report.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCurateListsConflicts(t *testing.T) {
	root, out := t.TempDir(), filepath.Join(t.TempDir(), "curated")
	writeEpisode(t, root, "a",
		row(1, 1, "human", `{"board":"open"}`, `{"cell":4}`),
		row(2, 1, "human", `{"board":"open"}`, `{"cell":0}`),
		row(3, 1, "human", `{"board":"open"}`, `{"cell":4}`),
	)
	rep, err := Curate(root, out, CurateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Conflicts) != 1 {
		t.Fatalf("conflicts %d, want 1", len(rep.Conflicts))
	}
	c := rep.Conflicts[0]
	if c.Rows != 3 || c.Actions[`{"cell":4}`] != 2 || c.Actions[`{"cell":0}`] != 1 {
		t.Fatalf("conflict %+v: want 3 rows, 2×cell4, 1×cell0", c)
	}
	// Curate surfaces the mixture and resolves nothing: every row stays.
	if rep.TrainKept != 3 {
		t.Fatalf("kept %d, want all 3", rep.TrainKept)
	}
}

func TestCurateSplitsWholeEpisodesDeterministically(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		writeEpisode(t, root, name,
			row(1, 1, "human", `{"board":"`+name+`"}`, `{"cell":1}`),
			row(2, 1, "human", `{"board":"`+name+`2"}`, `{"cell":2}`),
		)
	}
	out1 := filepath.Join(t.TempDir(), "curated")
	rep1, err := Curate(root, out1, CurateOptions{Holdout: 0.5, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	if rep1.HoldoutEpisodes == 0 || rep1.TrainEpisodes == 0 {
		t.Fatalf("split %d/%d put everything on one side", rep1.TrainEpisodes, rep1.HoldoutEpisodes)
	}
	if rep1.HoldoutRows != rep1.HoldoutEpisodes*2 {
		t.Fatalf("holdout rows %d for %d episodes: rows crossed the episode split", rep1.HoldoutRows, rep1.HoldoutEpisodes)
	}
	out2 := filepath.Join(t.TempDir(), "curated")
	rep2, err := Curate(root, out2, CurateOptions{Holdout: 0.5, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	if rep1.HoldoutEpisodes != rep2.HoldoutEpisodes || rep1.TrainKept != rep2.TrainKept {
		t.Fatalf("same inputs split differently: %+v vs %+v", rep1, rep2)
	}
}

func TestCurateRefusesForeignOutput(t *testing.T) {
	root := t.TempDir()
	writeEpisode(t, root, "a", row(1, 1, "human", `{"b":1}`, `{"c":1}`))
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "notes.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Curate(root, out, CurateOptions{}); err == nil {
		t.Fatal("curate overwrote a directory it did not write")
	}
	// A previous run's output is fair game.
	out2 := filepath.Join(t.TempDir(), "curated")
	if _, err := Curate(root, out2, CurateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := Curate(root, out2, CurateOptions{}); err != nil {
		t.Fatalf("re-curating over report.json: %v", err)
	}
}

func TestCurateResultFilterReadsOutcomes(t *testing.T) {
	root, out := t.TempDir(), filepath.Join(t.TempDir(), "curated")
	writeEpisode(t, root, "a",
		row(1, 1, "human", `{"b":1}`, `{"c":1}`),
		row(2, 2, "human", `{"b":2}`, `{"c":2}`),
	)
	// Without an outcomes stream the filter is an error, not a silent
	// empty match.
	if _, err := Curate(root, out, CurateOptions{Filter: RowFilter{Result: "win"}}); err == nil {
		t.Fatal("result filter without outcomes.jsonl should fail")
	}
	outcomes := `{"stream":"outcomes","schema_version":1,"episode_id":"a"}
{"slot":1,"result":"win","reward":0,"duration_ticks":9}
{"slot":2,"result":"loss","reward":0,"duration_ticks":9}
`
	if err := os.WriteFile(filepath.Join(root, "a", "outcomes.jsonl"), []byte(outcomes), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Curate(root, out, CurateOptions{Filter: RowFilter{Result: "win"}})
	if err != nil {
		t.Fatal(err)
	}
	if rep.TrainKept != 1 || rep.FilteredOut != 1 {
		t.Fatalf("kept %d filtered %d; want the winner's row alone", rep.TrainKept, rep.FilteredOut)
	}
}

func TestEvaluateBucketsAndGaps(t *testing.T) {
	v := &Vocabulary{
		Features: []Feature{{Name: "urgent"}, {Name: "calm"}},
		Actions:  []ActionDef{{Name: "strike"}, {Name: "wait"}},
	}
	lib := &Library{Chips: []Chip{
		{Condition: "urgent", Action: "strike", Priority: 0, Approved: true},
		{Condition: "calm", Action: "wait", Priority: 1, Approved: true},
	}}
	records := []Record{
		// urgent and struck: the first chip answers correctly.
		{Episode: "e1", Tick: 1, Action: "strike", Bits: []bool{true, false}},
		// urgent but waited: the first chip answers, wrongly — the
		// second never gets asked, exactly like the generated switch.
		{Episode: "e1", Tick: 2, Action: "wait", Bits: []bool{true, true}},
		// neither predicate holds: the policy is silent here.
		{Episode: "e2", Tick: 3, Slot: 1, Action: "wait", Bits: []bool{false, false}, Obs: json.RawMessage(`{"b":3}`)},
	}
	rep, err := Evaluate(v, lib, records)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Covered) != 1 || len(rep.Misplayed) != 1 || len(rep.Silent) != 1 {
		t.Fatalf("covered %d misplayed %d silent %d; want 1 each", len(rep.Covered), len(rep.Misplayed), len(rep.Silent))
	}

	var gaps strings.Builder
	if err := rep.WriteGaps(&gaps); err != nil {
		t.Fatal(err)
	}
	var gap struct {
		Episode string `json:"episode"`
		Tick    uint64 `json:"tick"`
		Action  string `json:"action"`
	}
	if err := json.Unmarshal([]byte(gaps.String()), &gap); err != nil {
		t.Fatal(err)
	}
	if gap.Episode != "e2" || gap.Tick != 3 || gap.Action != "wait" {
		t.Fatalf("gap %+v names the wrong moment", gap)
	}
}

func TestEvaluateRejectsStaleVocabulary(t *testing.T) {
	v := &Vocabulary{Features: []Feature{{Name: "urgent"}}}
	lib := &Library{Chips: []Chip{{Condition: "renamed", Action: "strike", Approved: true}}}
	if _, err := Evaluate(v, lib, []Record{{Bits: []bool{true}}}); err == nil {
		t.Fatal("a chip naming an unknown predicate must fail, as codegen does")
	}
}
