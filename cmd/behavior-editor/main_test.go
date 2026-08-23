package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/behavior"
)

func testServer(t *testing.T) (*server, string) {
	t.Helper()
	dir := t.TempDir()
	lib := &behavior.Library{Game: "test", Chips: []behavior.Chip{
		{Condition: "c1", Action: "a1", Coverage: 10,
			Evidence: []behavior.Evidence{{Episode: "ep-1", Tick: 3}}},
		{Condition: "c2", Action: "a2", Coverage: 5, Approved: true},
	}}
	path := filepath.Join(dir, "chips.json")
	if err := lib.Save(path); err != nil {
		t.Fatal(err)
	}
	// A one-episode corpus for the evidence pane.
	epDir := filepath.Join(dir, "ep-1")
	if err := os.MkdirAll(epDir, 0o755); err != nil {
		t.Fatal(err)
	}
	decisions := `{"stream":"decisions","schema_version":1,"episode_id":"ep-1","mode":"replay_complete","seed":1,"evaluation_version":0}
{"tick":3,"slot":1,"sight":{"Board":[0,1,0]},"action":{"Cell":2},"evaluation":{"score":0,"progress":0,"evaluation":0,"reward_delta":0}}
`
	if err := os.WriteFile(filepath.Join(epDir, "decisions.jsonl"), []byte(decisions), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, err := newServer(path, dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return srv, path
}

func TestLibraryAndMutations(t *testing.T) {
	srv, path := testServer(t)
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	// The page itself serves.
	page, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	page.Body.Close()
	if page.StatusCode != 200 {
		t.Fatalf("index status %d", page.StatusCode)
	}

	// Approve chip c1→a1 through the API; the file on disk changes.
	body := strings.NewReader(`{"key":"c1→a1","op":"approve"}`)
	rsp, err := http.Post(ts.URL+"/api/chip", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	rsp.Body.Close()
	if rsp.StatusCode != 200 {
		t.Fatalf("approve status %d", rsp.StatusCode)
	}
	onDisk, err := behavior.LoadLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	if !onDisk.Chips[0].Approved {
		t.Fatal("approval did not persist")
	}

	// Reject with a reason; the reason persists for regeneration diffs.
	body = strings.NewReader(`{"key":"c2→a2","op":"reject","reason":"too greedy"}`)
	rsp, _ = http.Post(ts.URL+"/api/chip", "application/json", body)
	rsp.Body.Close()
	onDisk, _ = behavior.LoadLibrary(path)
	if !onDisk.Chips[1].Rejected || onDisk.Chips[1].RejectReason != "too greedy" {
		t.Fatalf("rejection did not persist: %+v", onDisk.Chips[1])
	}

	// Evidence lookup returns the recorded moment.
	rsp, err = http.Get(ts.URL + "/api/evidence?episode=ep-1&tick=3")
	if err != nil {
		t.Fatal(err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != 200 {
		t.Fatalf("evidence status %d", rsp.StatusCode)
	}
	var row map[string]json.RawMessage
	if err := json.NewDecoder(rsp.Body).Decode(&row); err != nil {
		t.Fatal(err)
	}
	if string(row["action"]) != `{"Cell":2}` {
		t.Fatalf("evidence action = %s", row["action"])
	}
}
