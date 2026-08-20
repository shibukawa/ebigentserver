// Command behavior-merge folds analyzer proposals into a chip library.
// It is the pipeline side of the behavior-analyze skill's handoff: the
// proposals file — whether written by an LLM in the developer's own
// environment or by any other analyzer — is re-validated here against
// the analysis request (vocabulary membership, recomputed coverage,
// evidence existence) regardless of any validation that already
// happened, then merged as a diff that never silently overwrites an
// approved or rejected chip (rule:regeneration-preserves-approved-nodes).
//
//	behavior-merge -library chips.json -request analysis-request.json \
//	    -proposals validated-proposals.json [-diff diff.json]
//
// The printed diff (and optional -diff file, loadable in
// behavior-editor's diff tab) is what the developer reviews before
// approving anything (rule:generated-behavior-requires-approval).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/shibukawa/ebigentserver/behavior"
)

func main() {
	library := flag.String("library", "", "chip library JSON file (created if missing)")
	request := flag.String("request", "", "analysis-request.json the proposals answer")
	proposals := flag.String("proposals", "", "proposals JSON from the analyzer")
	diffOut := flag.String("diff", "", "optional path to write the diff JSON for behavior-editor")
	flag.Parse()
	if *library == "" || *request == "" || *proposals == "" {
		fmt.Fprintln(os.Stderr, "behavior-merge: -library, -request, and -proposals are required")
		os.Exit(2)
	}

	reqBytes, err := os.ReadFile(*request)
	if err != nil {
		fatal(err)
	}
	var req behavior.AnalysisRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		fatal(fmt.Errorf("request: %w", err))
	}
	props, err := behavior.LoadProposals(*proposals)
	if err != nil {
		fatal(err)
	}

	lib, err := behavior.LoadLibrary(*library)
	if os.IsNotExist(err) {
		lib = &behavior.Library{Game: req.Game}
	} else if err != nil {
		fatal(err)
	}

	valid, issues := behavior.ValidateProposals(req, props)
	for _, is := range issues {
		fmt.Printf("  [%s] %s: %s\n", is.Kind, is.Candidate, is.Detail)
	}
	diff := behavior.Merge(lib, valid)
	counts := map[behavior.DiffClass]int{}
	for _, d := range diff {
		counts[d.Class]++
		fmt.Printf("  %-24s %s → %s (coverage %d, counter %d)\n",
			d.Class, d.Candidate.Condition, d.Candidate.Action,
			d.Candidate.Coverage, d.Candidate.Counterexamples)
	}
	if err := lib.Save(*library); err != nil {
		fatal(err)
	}
	if *diffOut != "" {
		b, err := json.MarshalIndent(diff, "", " ")
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(*diffOut, append(b, '\n'), 0o644); err != nil {
			fatal(err)
		}
	}
	fmt.Printf("merged into %s: %d new, %d unchanged, %d metric changes, %d rejected-again, %d conflicts; %d validation issues\n",
		*library, counts[behavior.DiffNew], counts[behavior.DiffUnchanged], counts[behavior.DiffMetrics],
		counts[behavior.DiffRejectedAgain], counts[behavior.DiffConflict], len(issues))
	if len(props.Predicates) > 0 {
		fmt.Printf("%d new predicate proposals await a developer (implement and review them into the game's predicate package):\n", len(props.Predicates))
		for _, p := range props.Predicates {
			fmt.Printf("  %s — %s\n", p.Name, p.Doc)
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "behavior-merge:", err)
	os.Exit(1)
}
