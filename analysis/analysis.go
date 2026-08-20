// Package analysis computes metric:balance-signals aggregates over
// data:episode-log corpora: win rate by agent kind, duration
// distribution, action frequency, and rejection counts across many
// recorded episodes.
//
// It is analysis-side tooling under
// rule:analysis-tooling-outside-game-process: pure Go, stdlib only, no
// cgo — so it builds for every target including wasm, and it is never
// linked into a game or session process. The same rule keeps
// system:duckdb out of this binary; WriteDuckDBSQL instead emits a .sql
// file an operator runs in the duckdb CLI over the identical corpus.
package analysis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/shibukawa/ebigentserver/episode"
)

// DecisionRow is the retained slice of one data:decision-record row:
// enough for frequency and join analytics without holding the
// observation payloads of a large corpus in memory.
type DecisionRow struct {
	Tick      uint64
	Slot      uint16
	AgentKind string
	// HasAction distinguishes decision rows from observation-only
	// deliveries (a null/absent action in the stream).
	HasAction bool
}

// Episode is one episode directory's parsed summary.
type Episode struct {
	// Dir is the episode directory path.
	Dir string
	// Header is the stream header (schema, protocol, mode, seed),
	// taken from the decisions stream.
	Header episode.Header
	// Decisions retains one DecisionRow per data row of
	// decisions.jsonl.
	Decisions []DecisionRow
	// Outcomes holds every metric:episode-outcome row.
	Outcomes []episode.Outcome
	// EventKinds counts events.jsonl data rows by kind
	// ("lifecycle", "rejected", "checkpoint").
	EventKinds map[string]int
	// Rejections counts rejection events by reason.
	Rejections map[string]int
	// Checkpoints is the data:state-checkpoint row count.
	Checkpoints int
	// StreamBytes is the on-disk size of each present stream file,
	// keyed by stream name — the raw operations number behind
	// storage planning.
	StreamBytes map[string]int64
}

// Corpus is a loaded set of episodes under one root.
type Corpus struct {
	Root     string
	Episodes []Episode
}

// Stream file names inside one episode directory.
const (
	decisionsFile = "decisions.jsonl"
	eventsFile    = "events.jsonl"
	outcomesFile  = "outcomes.jsonl"
	worldFile     = "world.jsonl"
)

// LoadCorpus scans the subdirectories of root, each holding one
// episode's data:episode-log streams. A subdirectory is an episode when
// it contains decisions.jsonl; outcomes.jsonl is then required, while
// events.jsonl and world.jsonl may be absent (analysis_sampled corpora
// routinely drop the world stream). Episodes load in directory-name
// order, so the corpus — and everything Compute derives from it — is
// deterministic.
func LoadCorpus(root string) (*Corpus, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("analysis: read corpus root: %w", err)
	}
	c := &Corpus{Root: root}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, decisionsFile)); err != nil {
			continue // not an episode directory
		}
		ep, err := loadEpisode(dir)
		if err != nil {
			return nil, fmt.Errorf("analysis: episode %s: %w", dir, err)
		}
		c.Episodes = append(c.Episodes, ep)
	}
	if len(c.Episodes) == 0 {
		return nil, fmt.Errorf("analysis: no episodes under %s", root)
	}
	return c, nil
}

func loadEpisode(dir string) (Episode, error) {
	ep := Episode{
		Dir:         dir,
		EventKinds:  map[string]int{},
		Rejections:  map[string]int{},
		StreamBytes: map[string]int64{},
	}
	for _, name := range []string{decisionsFile, eventsFile, outcomesFile, worldFile} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		ep.StreamBytes[streamName(name)] = info.Size()
	}
	if err := ep.loadDecisions(filepath.Join(dir, decisionsFile)); err != nil {
		return ep, err
	}
	if err := ep.loadOutcomes(filepath.Join(dir, outcomesFile)); err != nil {
		return ep, err
	}
	// events.jsonl is optional; world.jsonl is sized above, never parsed
	// (ground truth is debugging data, not a balance input by default).
	if _, err := os.Stat(filepath.Join(dir, eventsFile)); err == nil {
		if err := ep.loadEvents(filepath.Join(dir, eventsFile)); err != nil {
			return ep, err
		}
	}
	return ep, nil
}

func streamName(file string) string {
	return file[:len(file)-len(".jsonl")]
}

// scanLines runs fn over every non-empty line of a JSONL file. The
// buffer cap admits large observation payloads.
func scanLines(path string, fn func(line []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			return fmt.Errorf("%s line %d: %w", filepath.Base(path), n, err)
		}
	}
	return sc.Err()
}

// header rows carry "stream"; data rows never do (see episode.Header vs
// the row types in episode/episode.go). The probe types below piggyback
// on that to split the two in one unmarshal.

func (ep *Episode) loadDecisions(path string) error {
	type probe struct {
		Stream    string          `json:"stream"`
		Tick      uint64          `json:"tick"`
		Slot      uint16          `json:"slot"`
		AgentKind string          `json:"agent_kind"`
		Action    json.RawMessage `json:"action"`
	}
	return scanLines(path, func(line []byte) error {
		var p probe
		if err := json.Unmarshal(line, &p); err != nil {
			return err
		}
		if p.Stream != "" { // header row
			return json.Unmarshal(line, &ep.Header)
		}
		ep.Decisions = append(ep.Decisions, DecisionRow{
			Tick:      p.Tick,
			Slot:      p.Slot,
			AgentKind: p.AgentKind,
			HasAction: len(p.Action) > 0 && string(p.Action) != "null",
		})
		return nil
	})
}

func (ep *Episode) loadOutcomes(path string) error {
	type probe struct {
		Stream string `json:"stream"`
		episode.Outcome
	}
	return scanLines(path, func(line []byte) error {
		var p probe
		if err := json.Unmarshal(line, &p); err != nil {
			return err
		}
		if p.Stream != "" {
			return nil // header row
		}
		ep.Outcomes = append(ep.Outcomes, p.Outcome)
		return nil
	})
}

func (ep *Episode) loadEvents(path string) error {
	type probe struct {
		Stream string `json:"stream"`
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	return scanLines(path, func(line []byte) error {
		var p probe
		if err := json.Unmarshal(line, &p); err != nil {
			return err
		}
		if p.Stream != "" {
			return nil // header row
		}
		ep.EventKinds[p.Kind]++
		switch p.Kind {
		case "rejected":
			ep.Rejections[p.Reason]++
		case "checkpoint":
			ep.Checkpoints++
		}
		return nil
	})
}

// AgentKinds joins the episode's decision rows into a slot → agent_kind
// table (first non-empty kind wins), the join key that turns per-slot
// outcomes into per-kind metric:balance-signals.
func (ep *Episode) AgentKinds() map[uint16]string {
	kinds := map[uint16]string{}
	for _, d := range ep.Decisions {
		if d.AgentKind != "" {
			if _, seen := kinds[d.Slot]; !seen {
				kinds[d.Slot] = d.AgentKind
			}
		}
	}
	return kinds
}

// sortedKeys returns a map's keys in sorted order, for deterministic
// report output.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
