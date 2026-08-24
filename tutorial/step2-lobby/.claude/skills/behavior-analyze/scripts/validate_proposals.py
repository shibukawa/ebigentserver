#!/usr/bin/env python3
"""Validate analyzer proposals against an analysis request.

This is the trust boundary of the behavior-analyze skill, mirroring the
framework's behavior.ValidateProposals exactly: conditions and actions
must exist in the exported vocabulary (the mechanical form of
rule:analysis-restricted-to-visible-fields), and every number a proposal
claims is recomputed from the featurized records under decision-list
semantics. Claimed coverage is discarded; a proposal is advice, never
authority.

Stdlib only, on purpose: sharing the skill folder is the whole install.

Usage:
  validate_proposals.py --request analysis-request.json \
      --proposals proposals.json --out validated-proposals.json

Exit status: 0 when every candidate validated cleanly, 1 when any was
dropped or corrected (the report on stdout says which and why), 2 on
malformed input.
"""

import argparse
import json
import sys


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def validate(request, proposals):
    feat_idx = {f["name"]: i for i, f in enumerate(request.get("features", []))}
    actions = {a["name"] for a in request.get("actions", [])}
    records = [dict(r, taken=False) for r in request.get("records", [])]
    moments = {(r["episode"], r["tick"]) for r in records}

    valid, issues = [], []
    for cand in proposals.get("candidates", []):
        key = "%s→%s" % (cand.get("condition"), cand.get("action"))
        fi = feat_idx.get(cand.get("condition"))
        if fi is None:
            issues.append((key, "unknown_condition",
                           "predicate %r is not in the vocabulary; the runtime agent "
                           "could never evaluate it" % cand.get("condition")))
            continue
        if cand.get("action") not in actions:
            issues.append((key, "unknown_action",
                           "action %r is not in the vocabulary" % cand.get("action")))
            continue

        coverage, counter, evidence, matched = 0, 0, [], []
        for rec in records:
            bits = rec.get("bits", "")
            if rec["taken"] or fi >= len(bits) or bits[fi] != "1":
                continue
            matched.append(rec)
            if rec.get("action") == cand["action"]:
                coverage += 1
                if len(evidence) < 5:
                    evidence.append({"episode": rec["episode"], "tick": rec["tick"]})
            else:
                counter += 1

        if coverage == 0:
            # A dropped rule does not exist in the final list, so it must
            # not consume records from the rules after it.
            issues.append((key, "no_coverage",
                           "no remaining record supports this rule at its list position"))
            continue
        for rec in matched:
            rec["taken"] = True  # decision list: an accepted rule handles these from here on
        claimed_cov = cand.get("coverage", 0)
        if claimed_cov and (claimed_cov != coverage or
                            cand.get("counterexamples", 0) != counter):
            issues.append((key, "coverage_corrected",
                           "claimed %d/%d, recomputed %d/%d" %
                           (claimed_cov, cand.get("counterexamples", 0), coverage, counter)))
        for ev in cand.get("evidence", []):
            if (ev.get("episode"), ev.get("tick")) not in moments:
                issues.append((key, "evidence_invalid",
                               "cited moment %s@%s is not in the corpus" %
                               (ev.get("episode"), ev.get("tick"))))

        valid.append({
            "condition": cand["condition"],
            "action": cand["action"],
            "priority": len(valid),
            "coverage": coverage,
            "counterexamples": counter,
            "evidence": evidence,
            "rationale": cand.get("rationale", ""),
        })
    uncovered = sum(1 for r in records if not r["taken"])
    return valid, issues, uncovered


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--request", required=True)
    ap.add_argument("--proposals", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    try:
        request = load(args.request)
        proposals = load(args.proposals)
    except (OSError, json.JSONDecodeError) as e:
        print("validate_proposals: %s" % e, file=sys.stderr)
        return 2

    valid, issues, uncovered = validate(request, proposals)

    out = {
        "game": proposals.get("game", request.get("game", "")),
        "candidates": valid,
        "predicates": proposals.get("predicates", []),
        "notes": proposals.get("notes", ""),
    }
    with open(args.out, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=1)
        f.write("\n")

    total_cov = sum(c["coverage"] for c in valid)
    print("validated %d/%d candidates; covered %d records, %d counterexamples booked, %d records uncovered"
          % (len(valid), len(proposals.get("candidates", [])), total_cov,
             sum(c["counterexamples"] for c in valid), uncovered))
    for key, kind, detail in issues:
        print("  [%s] %s: %s" % (kind, key, detail))
    return 1 if issues else 0


if __name__ == "__main__":
    sys.exit(main())
