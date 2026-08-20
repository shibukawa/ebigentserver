#!/usr/bin/env python3
"""Corpus exploration for the behavior-analyze skill.

Summarizes an analysis-request.json (vocabulary, per-action counts,
feature/action co-occurrence — the raw material for spotting
situation→action patterns) and, when a corpus root is available, digs
into the episode JSONL streams through the **duckdb CLI** (never a
library binding: install the duckdb binary and share this folder, that
is the whole setup). Falls back to a pure-Python scan when duckdb is not
on PATH, so the skill degrades instead of failing.

Stdlib only.

Usage:
  corpus_stats.py --request analysis-request.json            # summary
  corpus_stats.py --request analysis-request.json --cooccurrence
  corpus_stats.py --corpus DIR --sql "SELECT ..."            # duckdb query
  corpus_stats.py --corpus DIR --outcomes                    # canned reports
"""

import argparse
import collections
import glob
import json
import os
import shutil
import subprocess
import sys


def load_request(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def summarize(request):
    records = request.get("records", [])
    features = request.get("features", [])
    print("game: %s" % request.get("game"))
    print("features: %d, actions: %d, records: %d"
          % (len(features), len(request.get("actions", [])), len(records)))
    by_action = collections.Counter(r.get("action") for r in records)
    print("\nrecords per action:")
    for action, n in by_action.most_common():
        print("  %-24s %6d" % (action, n))
    lib = request.get("library") or {}
    chips = lib.get("chips") or []
    if chips:
        print("\nexisting library: %d chips (%d approved, %d rejected)"
              % (len(chips),
                 sum(1 for c in chips if c.get("approved")),
                 sum(1 for c in chips if c.get("rejected"))))


def cooccurrence(request, top):
    """For each feature: how records split across actions when it holds.

    A feature whose 'on' records concentrate on one action is a chip
    candidate; the purity column says how concentrated.
    """
    records = request.get("records", [])
    features = request.get("features", [])
    rows = []
    for fi, f in enumerate(features):
        by_action = collections.Counter()
        for r in records:
            bits = r.get("bits", "")
            if fi < len(bits) and bits[fi] == "1":
                by_action[r.get("action")] += 1
        total = sum(by_action.values())
        if total == 0:
            continue
        action, best = by_action.most_common(1)[0]
        rows.append((best / total, total, f["name"], action, best))
    rows.sort(key=lambda r: (-r[0], -r[1], r[2]))
    print("%-28s %-20s %8s %8s %7s" % ("feature", "top action", "matches", "agree", "purity"))
    for purity, total, name, action, best in rows[:top]:
        print("%-28s %-20s %8d %8d %6.1f%%" % (name, action, total, best, purity * 100))


def duckdb_available():
    return shutil.which("duckdb") is not None


def run_duckdb(corpus, sql):
    """Run one query in the duckdb CLI over the corpus JSONL streams."""
    views = []
    for stream in ("decisions", "events", "outcomes"):
        pattern = os.path.join(corpus, "*", stream + ".jsonl")
        if glob.glob(pattern):
            views.append(
                "CREATE VIEW %s AS SELECT * FROM read_json_auto('%s', format='newline_delimited') "
                "WHERE stream IS NULL;" % (stream, pattern))
    script = "\n".join(views) + "\n" + sql
    proc = subprocess.run(["duckdb", "-json", "-c", script],
                          capture_output=True, text=True)
    if proc.returncode != 0:
        print(proc.stderr.strip(), file=sys.stderr)
        return proc.returncode
    print(proc.stdout.strip())
    return 0


def python_outcomes(corpus):
    """Fallback outcome tally when duckdb is absent."""
    tally = collections.Counter()
    for path in sorted(glob.glob(os.path.join(corpus, "*", "outcomes.jsonl"))):
        with open(path, encoding="utf-8") as f:
            for line in f:
                row = json.loads(line)
                if "stream" in row:
                    continue
                tally[(row.get("slot"), row.get("result"))] += 1
    for (slot, result), n in sorted(tally.items(), key=lambda kv: (kv[0][0] or 0, kv[0][1] or "")):
        print("slot %-4s %-12s %6d" % (slot, result, n))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--request")
    ap.add_argument("--cooccurrence", action="store_true")
    ap.add_argument("--top", type=int, default=40)
    ap.add_argument("--corpus")
    ap.add_argument("--sql")
    ap.add_argument("--outcomes", action="store_true")
    args = ap.parse_args()

    if args.request:
        request = load_request(args.request)
        if args.cooccurrence:
            cooccurrence(request, args.top)
        else:
            summarize(request)
        return 0

    if args.corpus:
        if args.sql:
            if not duckdb_available():
                print("duckdb CLI not on PATH; only --outcomes fallback is available",
                      file=sys.stderr)
                return 2
            return run_duckdb(args.corpus, args.sql)
        if args.outcomes:
            if duckdb_available():
                return run_duckdb(
                    args.corpus,
                    "SELECT slot, result, count(*) AS n FROM outcomes GROUP BY slot, result ORDER BY slot, result;")
            python_outcomes(args.corpus)
            return 0

    print("nothing to do: pass --request or --corpus (see --help)", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
