// Command corpus-report prints metric:balance-signals aggregates over a
// data:episode-log corpus and optionally emits a system:duckdb SQL
// script for the deeper cuts.
//
// It is a standalone analysis tool under
// rule:analysis-tooling-outside-game-process: it reads recorded files
// after the fact and never links into a game or session process, and it
// carries no cgo — the DuckDB step is a generated .sql file the
// operator runs in the duckdb CLI.
//
// Usage:
//
//	corpus-report -corpus DIR [-sql FILE]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shibukawa/ebigentserver/analysis"
)

func main() {
	corpus := flag.String("corpus", "", "corpus root: one subdirectory per episode (required)")
	sqlOut := flag.String("sql", "", "also write a DuckDB report script to this file (run: duckdb -init FILE)")
	flag.Parse()
	if *corpus == "" {
		fmt.Fprintln(os.Stderr, "corpus-report: -corpus DIR is required")
		flag.Usage()
		os.Exit(2)
	}

	c, err := analysis.LoadCorpus(*corpus)
	if err != nil {
		fmt.Fprintln(os.Stderr, "corpus-report:", err)
		os.Exit(1)
	}
	analysis.Compute(c).WriteText(os.Stdout)

	if *sqlOut != "" {
		f, err := os.Create(*sqlOut)
		if err != nil {
			fmt.Fprintln(os.Stderr, "corpus-report:", err)
			os.Exit(1)
		}
		werr := analysis.WriteDuckDBSQL(f, *corpus)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			fmt.Fprintln(os.Stderr, "corpus-report:", werr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "\nduckdb script written: %s (run: duckdb -init %s)\n", *sqlOut, *sqlOut)
	}
}
