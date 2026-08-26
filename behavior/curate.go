package behavior

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/shibukawa/ebigentserver/episode"
)

// Curate is the curate step of flow:behavior-tree-synthesis
// (requirement:corpus-curation): it reads a recorded corpus and writes a
// filtered, deduplicated, split copy for distillation to mine
// (decision:curate-corpus-to-corpus).
//
// It is a corpus→corpus transform on purpose. The situation key is the
// recorded sight itself, not the feature bits a vocabulary would assign:
// a weak vocabulary maps distinct situations to the same bits, and
// deduplicating on those would merge counts that are not the same
// situation. Keying on what actually happened keeps curation honest
// about corpora the current vocabulary cannot yet describe — and keeps
// the intermediate a directory of JSONL a person can open and diff.
//
// Downstream stays untouched: Segment, the analyzer, Merge, and codegen
// read the curated corpus exactly as they read a recorded one.

// RowFilter selects rows of the decisions stream. The zero value keeps
// everything; each set field narrows.
type RowFilter struct {
	// AgentKind keeps rows whose agent_kind column matches: "human"
	// for a person, a policy name for a bot (data:decision-record).
	// This is the filter that separates a human corpus from the bot
	// rows sharing its episodes.
	AgentKind string `json:"agent_kind,omitempty"`
	// Slot keeps one seat; nil keeps every seat.
	Slot *uint16 `json:"slot,omitempty"`
	// Result keeps rows of seats whose episode ended with this
	// outcome ("win", "draw", ...), read from the outcomes stream.
	Result string `json:"result,omitempty"`
}

// keep applies the filter to one row; result is the outcomes-stream
// verdict for the row's slot, empty when the stream has none.
func (f RowFilter) keep(row episode.Decision, result string) bool {
	if f.AgentKind != "" && row.AgentKind != f.AgentKind {
		return false
	}
	if f.Slot != nil && row.Slot != *f.Slot {
		return false
	}
	if f.Result != "" && result != f.Result {
		return false
	}
	return true
}

// CurateOptions shape one curation run. Every field is echoed into the
// report, so a curated corpus reproduces from its inputs.
type CurateOptions struct {
	Filter RowFilter `json:"filter"`
	// Cap bounds how many rows one (situation, action) pair may keep
	// in the training set; 0 keeps all. Retention is by copies rather
	// than weights so coverage keeps meaning "supporting records" and
	// every retained row still names a replayable episode and tick.
	Cap int `json:"cap"`
	// Holdout is the fraction of episodes reserved for evaluation.
	// The split is by whole episodes: adjacent ticks of one match are
	// near-duplicates, and splitting rows would leak them across.
	Holdout float64 `json:"holdout"`
	// Seed makes the episode split deterministic.
	Seed uint64 `json:"seed"`
}

// Conflict is one situation the corpus answers in more than one way.
// The deterministic analyzer will treat the minority as counterexamples
// (concept:behavior-evidence), so curate lists these before mining does:
// whether the mixture is two policies to separate or one person's
// inconsistency is a judgement, and judgements belong to the developer
// (rule:generated-behavior-requires-approval), not to a majority vote.
type Conflict struct {
	Sight json.RawMessage `json:"sight"`
	// Actions counts rows per action, keyed by the compact action JSON.
	Actions map[string]int `json:"actions"`
	Rows    int            `json:"rows"`
}

// Duplicate is one situation by how often the training rows repeat it.
type Duplicate struct {
	Sight json.RawMessage `json:"sight"`
	Rows  int             `json:"rows"`
	Kept  int             `json:"kept"`
}

// CurateReport says what one run read, dropped, and wrote — and why the
// numbers moved. It is written to report.json beside the curated corpus.
type CurateReport struct {
	Source  string        `json:"source"`
	Out     string        `json:"out"`
	Options CurateOptions `json:"options"`

	Episodes        int `json:"episodes"`
	TrainEpisodes   int `json:"train_episodes"`
	HoldoutEpisodes int `json:"holdout_episodes"`

	// Read counts decision rows; SightOnly the rows with no action
	// (delivered sights, never mined); FilteredOut what RowFilter
	// rejected.
	Read        int `json:"read"`
	SightOnly   int `json:"sight_only"`
	FilteredOut int `json:"filtered_out"`

	// TrainRows survived the filter; TrainKept survived the cap too.
	TrainRows    int `json:"train_rows"`
	TrainKept    int `json:"train_kept"`
	TrainDropped int `json:"train_dropped"`
	HoldoutRows  int `json:"holdout_rows"`

	// Situations counts distinct sights among the training rows.
	Situations    int         `json:"situations"`
	Conflicts     []Conflict  `json:"conflicts,omitempty"`
	TopDuplicates []Duplicate `json:"top_duplicates,omitempty"`
}

// WriteText renders the report the way a person reads it at the
// terminal; report.json carries the full detail.
func (r CurateReport) WriteText(w io.Writer) {
	fmt.Fprintf(w, "curate: %d decisions in %d episodes from %s\n", r.Read, r.Episodes, r.Source)
	fmt.Fprintf(w, "filter: %d rows kept (%d filtered out, %d sight-only)\n",
		r.TrainRows+r.HoldoutRows, r.FilteredOut, r.SightOnly)
	if r.Options.Holdout > 0 {
		fmt.Fprintf(w, "split: %d train / %d holdout episodes (holdout %.2f, seed %d)\n",
			r.TrainEpisodes, r.HoldoutEpisodes, r.Options.Holdout, r.Options.Seed)
	}
	fmt.Fprintf(w, "situations: %d distinct among %d training rows\n", r.Situations, r.TrainRows)
	if len(r.TopDuplicates) > 0 {
		d := r.TopDuplicates[0]
		fmt.Fprintf(w, "most repeated situation: %d rows, %d kept\n", d.Rows, d.Kept)
	}
	if r.Options.Cap > 0 {
		fmt.Fprintf(w, "cap %d: %d of %d training rows kept (%d dropped)\n",
			r.Options.Cap, r.TrainKept, r.TrainRows, r.TrainDropped)
	}
	fmt.Fprintf(w, "conflicts: %d situations answered with more than one action\n", len(r.Conflicts))
	fmt.Fprintf(w, "curated corpus written: %s\n", r.Out)
}

// curatedRow is one decision row on its way through: the decoded columns
// for filtering and keying, and the original line for writing back. The
// bytes the recorder wrote go out verbatim — re-marshalling would make
// this stage a schema in its own right, and it is not one.
type curatedRow struct {
	raw []byte
	sit string // compact sight JSON, the situation key
	act string // compact action JSON
}

// curatedEpisode is one episode's surviving part.
type curatedEpisode struct {
	name    string
	header  []byte
	rows    []curatedRow
	holdout bool
}

// Curate reads the corpus under root and writes train/, holdout/, and
// report.json under out. It refuses an out directory it did not write:
// the only directories it deletes are a previous run's own output.
func Curate(root, out string, opts CurateOptions) (CurateReport, error) {
	rep := CurateReport{Source: root, Out: out, Options: opts}
	entries, err := os.ReadDir(root)
	if err != nil {
		return rep, err
	}

	var eps []*curatedEpisode
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ep, err := curateEpisode(root, e.Name(), opts, &rep)
		if err != nil {
			return rep, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if ep != nil {
			eps = append(eps, ep)
		}
	}
	if len(eps) == 0 {
		return rep, ErrEmptyCorpus
	}

	// Aggregate, list conflicts, and cap — training episodes only. The
	// holdout keeps its true distribution: it exists to measure the
	// mined policy, and a measurement taken on curated rows would be a
	// measurement of the curation.
	sits := map[string]*Duplicate{}
	acts := map[string]map[string]int{}
	counts := map[string]int{}
	for _, ep := range eps {
		if ep.holdout {
			rep.HoldoutRows += len(ep.rows)
			continue
		}
		rep.TrainRows += len(ep.rows)
		var kept []curatedRow
		for _, row := range ep.rows {
			d := sits[row.sit]
			if d == nil {
				d = &Duplicate{Sight: json.RawMessage(row.sit)}
				sits[row.sit] = d
				acts[row.sit] = map[string]int{}
			}
			d.Rows++
			acts[row.sit][row.act]++
			key := row.sit + "\x00" + row.act
			counts[key]++
			if opts.Cap > 0 && counts[key] > opts.Cap {
				rep.TrainDropped++
				continue
			}
			d.Kept++
			kept = append(kept, row)
		}
		ep.rows = kept
		rep.TrainKept += len(kept)
	}

	rep.Situations = len(sits)
	for sit, d := range sits {
		if len(acts[sit]) > 1 {
			rep.Conflicts = append(rep.Conflicts, Conflict{Sight: d.Sight, Actions: acts[sit], Rows: d.Rows})
		}
		rep.TopDuplicates = append(rep.TopDuplicates, *d)
	}
	sort.Slice(rep.Conflicts, func(i, j int) bool {
		if rep.Conflicts[i].Rows != rep.Conflicts[j].Rows {
			return rep.Conflicts[i].Rows > rep.Conflicts[j].Rows
		}
		return string(rep.Conflicts[i].Sight) < string(rep.Conflicts[j].Sight)
	})
	sort.Slice(rep.TopDuplicates, func(i, j int) bool {
		if rep.TopDuplicates[i].Rows != rep.TopDuplicates[j].Rows {
			return rep.TopDuplicates[i].Rows > rep.TopDuplicates[j].Rows
		}
		return string(rep.TopDuplicates[i].Sight) < string(rep.TopDuplicates[j].Sight)
	})
	if len(rep.TopDuplicates) > 5 {
		rep.TopDuplicates = rep.TopDuplicates[:5]
	}

	if err := writeCurated(out, eps, rep); err != nil {
		return rep, err
	}
	return rep, nil
}

// curateEpisode reads one episode directory, filters its rows, and
// assigns it a side of the split. A nil episode had nothing to keep.
func curateEpisode(root, name string, opts CurateOptions, rep *CurateReport) (*curatedEpisode, error) {
	f, err := os.Open(filepath.Join(root, name, "decisions.jsonl"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	results, err := readOutcomes(root, name, opts.Filter.Result != "")
	if err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	if !sc.Scan() {
		return nil, sc.Err() // empty stream: nothing recorded
	}
	rep.Episodes++
	ep := &curatedEpisode{
		name:    name,
		header:  append([]byte(nil), sc.Bytes()...),
		holdout: opts.Holdout > 0 && splitFraction(name, opts.Seed) < opts.Holdout,
	}
	if ep.holdout {
		rep.HoldoutEpisodes++
	} else {
		rep.TrainEpisodes++
	}

	for line := 2; sc.Scan(); line++ {
		var row episode.Decision
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			return nil, fmt.Errorf("decisions line %d: %w", line, err)
		}
		rep.Read++
		if len(row.Action) == 0 || string(row.Action) == "null" {
			rep.SightOnly++
			continue
		}
		if !opts.Filter.keep(row, results[row.Slot]) {
			rep.FilteredOut++
			continue
		}
		sit, err := compactJSON(row.Sight)
		if err != nil {
			return nil, fmt.Errorf("decisions line %d: sight: %w", line, err)
		}
		act, err := compactJSON(row.Action)
		if err != nil {
			return nil, fmt.Errorf("decisions line %d: action: %w", line, err)
		}
		ep.rows = append(ep.rows, curatedRow{
			raw: append([]byte(nil), sc.Bytes()...),
			sit: sit,
			act: act,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(ep.rows) == 0 {
		return nil, nil
	}
	return ep, nil
}

// readOutcomes loads the episode's outcomes stream when a result filter
// needs it. A result filter over an episode that recorded no outcomes is
// an error rather than an empty match: silently keeping nothing would
// read as "nobody won", which is not what happened.
func readOutcomes(root, name string, required bool) (map[uint16]string, error) {
	f, err := os.Open(filepath.Join(root, name, "outcomes.jsonl"))
	if os.IsNotExist(err) {
		if required {
			return nil, fmt.Errorf("result filter needs outcomes.jsonl, which this episode did not record")
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[uint16]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	if !sc.Scan() {
		return out, sc.Err() // header or nothing
	}
	for sc.Scan() {
		var o episode.Outcome
		if err := json.Unmarshal(sc.Bytes(), &o); err != nil {
			return nil, fmt.Errorf("outcomes: %w", err)
		}
		out[o.Slot] = o.Result
	}
	return out, sc.Err()
}

// splitFraction maps an episode name and seed to [0, 1). Hashing the
// name rather than counting keeps the assignment stable when episodes
// are added: yesterday's holdout episodes stay held out.
func splitFraction(name string, seed uint64) float64 {
	h := fnv.New64a()
	io.WriteString(h, name)
	fmt.Fprintf(h, "|%d", seed)
	return float64(h.Sum64()%1_000_000) / 1_000_000
}

// writeCurated puts the surviving rows on disk. It only ever deletes a
// directory a previous run wrote, which report.json marks; anything
// else in the way is somebody's data and stays an error.
func writeCurated(out string, eps []*curatedEpisode, rep CurateReport) error {
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		if _, err := os.Stat(filepath.Join(out, "report.json")); err != nil {
			return fmt.Errorf("curate: %s exists and is not a curated corpus; refusing to overwrite it", out)
		}
	}
	for _, side := range []string{"train", "holdout"} {
		if err := os.RemoveAll(filepath.Join(out, side)); err != nil {
			return err
		}
	}
	for _, ep := range eps {
		side := "train"
		if ep.holdout {
			side = "holdout"
		}
		dir := filepath.Join(out, side, ep.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		var b bytes.Buffer
		b.Write(ep.header)
		b.WriteByte('\n')
		for _, row := range ep.rows {
			b.Write(row.raw)
			b.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, "decisions.jsonl"), b.Bytes(), 0o644); err != nil {
			return err
		}
	}
	body, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "report.json"), append(body, '\n'), 0o644)
}

// compactJSON canonicalizes a recorded value into a map key. One
// recorder wrote every line, so compaction is the only normalization the
// key needs.
func compactJSON(raw json.RawMessage) (string, error) {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return "", err
	}
	return b.String(), nil
}
