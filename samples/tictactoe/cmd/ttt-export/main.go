// Command ttt-export produces the behavior-analyze skill's inputs for
// tic-tac-toe: a recorded corpus of episode directories and the
// analysis-request.json describing the vocabulary and featurized
// decisions. The output feeds an external analyzer working in its own
// environment; nothing it proposes enters the pipeline without
// re-validation (behavior.ValidateProposals via cmd/behavior-merge).
//
//	ttt-export -out DIR [-matches 200] [-library chips.json]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shibukawa/ebigentserver/behavior"
	"github.com/shibukawa/ebigentserver/samples/tictactoe/distill"
)

func main() {
	out := flag.String("out", "", "output directory (required)")
	matches := flag.Int("matches", 200, "matches to record")
	library := flag.String("library", "", "existing chip library to embed for diff-aware analysis")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "ttt-export: -out is required")
		os.Exit(2)
	}
	corpusDir := filepath.Join(*out, "corpus")
	records, err := distill.ExportCorpus(corpusDir, *matches)
	if err != nil {
		fatal(err)
	}
	var lib *behavior.Library
	if *library != "" {
		if lib, err = behavior.LoadLibrary(*library); err != nil {
			fatal(err)
		}
	}
	req := behavior.BuildAnalysisRequest("tictactoe", distill.Vocabulary(), records, lib, corpusDir)
	reqPath := filepath.Join(*out, "analysis-request.json")
	if err := req.Save(reqPath); err != nil {
		fatal(err)
	}
	fmt.Printf("exported %d episodes (%d decisions) to %s\nrequest: %s\n",
		*matches, len(records), corpusDir, reqPath)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ttt-export:", err)
	os.Exit(1)
}
