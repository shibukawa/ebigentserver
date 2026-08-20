// Command behavior-editor is ui:behavior-tree-editor in its first usable
// form: a local web UI over one chip library file. The panes follow the
// concept — the chip list, the candidate detail with accept/reject, the
// evidence view (real recorded situations, the point of the tool), the
// predicate vocabulary usage, the level matrix over tags, and the
// regeneration diff — plus a benchmark tab (ui:chip-benchmark's summary
// table) fed from a matchloop results file.
//
//	behavior-editor -library chips.json [-corpus DIR] [-diff diff.json] [-bench bench.json]
//
// Approve and reject write straight back to the library file; the
// developer gate of rule:generated-behavior-requires-approval is this
// screen. Evidence is read from a corpus directory holding one
// subdirectory per episode with its decisions.jsonl.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	_ "embed"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/episode"
)

//go:embed editor.html
var editorHTML []byte

func main() {
	library := flag.String("library", "", "chip library JSON file (required)")
	corpus := flag.String("corpus", "", "episode corpus root for the evidence pane")
	diffFile := flag.String("diff", "", "regeneration diff JSON to display")
	benchFile := flag.String("bench", "", "matchloop benchmark JSON to display")
	addr := flag.String("addr", "127.0.0.1:8931", "listen address (loopback by default)")
	flag.Parse()
	if *library == "" {
		fmt.Fprintln(os.Stderr, "behavior-editor: -library is required")
		os.Exit(1)
	}
	srv, err := newServer(*library, *corpus, *diffFile, *benchFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "behavior-editor:", err)
		os.Exit(1)
	}
	fmt.Printf("behavior-editor: http://%s (library %s)\n", *addr, *library)
	if err := http.ListenAndServe(*addr, srv.mux()); err != nil {
		fmt.Fprintln(os.Stderr, "behavior-editor:", err)
		os.Exit(1)
	}
}

type server struct {
	mu        sync.Mutex
	path      string
	lib       *behavior.Library
	corpus    string
	diffFile  string
	benchFile string
}

func newServer(library, corpus, diffFile, benchFile string) (*server, error) {
	lib, err := behavior.LoadLibrary(library)
	if err != nil {
		return nil, err
	}
	return &server{path: library, lib: lib, corpus: corpus, diffFile: diffFile, benchFile: benchFile}, nil
}

func (s *server) mux() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(editorHTML)
	})
	m.HandleFunc("GET /api/library", s.getLibrary)
	m.HandleFunc("POST /api/chip", s.postChip)
	m.HandleFunc("GET /api/evidence", s.getEvidence)
	m.HandleFunc("GET /api/diff", s.fileJSON(func() string { return s.diffFile }))
	m.HandleFunc("GET /api/bench", s.fileJSON(func() string { return s.benchFile }))
	return m
}

func (s *server) getLibrary(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, s.lib)
}

// chipOp is the mutation request: op approve|reject|clear|tags.
type chipOp struct {
	Key    string   `json:"key"`
	Op     string   `json:"op"`
	Reason string   `json:"reason,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

func (s *server) postChip(w http.ResponseWriter, r *http.Request) {
	var req chipOp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var chip *behavior.Chip
	for i := range s.lib.Chips {
		if s.lib.Chips[i].Key() == req.Key {
			chip = &s.lib.Chips[i]
		}
	}
	if chip == nil {
		http.Error(w, "unknown chip", http.StatusNotFound)
		return
	}
	switch req.Op {
	case "approve":
		chip.Approved, chip.Rejected, chip.RejectReason = true, false, ""
	case "reject":
		chip.Approved, chip.Rejected, chip.RejectReason = false, true, req.Reason
	case "clear":
		chip.Approved, chip.Rejected, chip.RejectReason = false, false, ""
	case "tags":
		chip.Tags = req.Tags
	default:
		http.Error(w, "unknown op", http.StatusBadRequest)
		return
	}
	if err := s.lib.Save(s.path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, chip)
}

// getEvidence finds one recorded decision row: the moment a chip's rule
// fired (concept:behavior-evidence made concrete).
func (s *server) getEvidence(w http.ResponseWriter, r *http.Request) {
	if s.corpus == "" {
		http.Error(w, "no corpus configured", http.StatusNotFound)
		return
	}
	ep, tick := r.URL.Query().Get("episode"), r.URL.Query().Get("tick")
	f, err := os.Open(filepath.Join(s.corpus, filepath.Base(ep), "decisions.jsonl"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		var row episode.Decision
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		if fmt.Sprint(row.Tick) == tick && len(row.Action) > 0 {
			writeJSON(w, row)
			return
		}
	}
	http.Error(w, "moment not found", http.StatusNotFound)
}

func (s *server) fileJSON(path func() string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := path()
		if p == "" {
			writeJSON(w, nil)
			return
		}
		b, err := os.ReadFile(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
