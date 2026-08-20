---
name: behavior-analyze
description: Analyze a recorded gameplay corpus and propose behavior chips (condition→action rules with evidence) for the ebigentserver distillation pipeline. Use when the user asks to mine episodes, propose chips or predicates, or run the LLM analysis step of behavior-tree synthesis on an analysis-request.json.
---

# Behavior Analyze

You are the analysis step of `flow:behavior-tree-synthesis`: read a
featurized gameplay corpus, find recurring situation→action patterns,
and propose chips the developer will review. You propose; the pipeline
verifies; the developer decides. Nothing you write becomes behavior
without passing the validator and the human approval gate.

Setup on the user's machine is exactly two things: this folder, and
(optionally, for deep corpus queries) the `duckdb` CLI on PATH. The
scripts are Python stdlib only — never install packages.

## Inputs and outputs

- Input: `analysis-request.json`, exported by the game's pipeline
  (`behavior.BuildAnalysisRequest` in Go). It carries:
  - `features`: the predicate vocabulary — name, prose doc, and the Go
    expression the generated code will use. **You may only write
    conditions over these names.** Anything else is invisible to the
    runtime agent and will be rejected.
  - `actions`: the action vocabulary, same deal.
  - `records`: one row per recorded decision — episode, tick, slot, the
    action taken, and `bits` (one `0`/`1` per feature, in feature
    order). The bits are the visible facts.
  - `library` (optional): existing chips, so you can propose diffs
    instead of rediscovering approved rules and can avoid re-proposing
    rejected ones (their `reject_reason` tells you why).
  - `corpus_root` (optional): the raw episode JSONL directories for
    deeper digging.
- Output: `proposals.json`:

```json
{
 "game": "<from the request>",
 "candidates": [
  {"condition": "<feature name>", "action": "<action name>",
   "rationale": "why this rule, in one or two sentences",
   "evidence": [{"episode": "ep-001", "tick": 12}]}
 ],
 "predicates": [
  {"name": "new_predicate_name", "doc": "what it judges and why it is needed",
   "go_draft": "func NewPredicate(obs game.Observation) bool { ... }",
   "rationale": "which decisions the current vocabulary cannot separate"}
 ],
 "notes": "summary for the reviewer"
}
```

Candidates use **decision-list semantics**: order matters, earlier rules
take precedence, so a later condition needs no negations of earlier
ones. Put the highest-coverage cleanest rules first. Do not bother
computing coverage numbers — the validator recomputes them and discards
whatever you claim; spend your effort on choosing conditions and writing
honest rationales.

## Workflow

1. **Survey.** `python3 scripts/corpus_stats.py --request analysis-request.json`
   for sizes and per-action counts, then
   `--cooccurrence` for the feature×action purity table — features whose
   matches concentrate on one action are chip material; features with
   low purity need a sharper predicate (that is a `predicates` proposal,
   not a dirty chip).
2. **Dig where needed.** With `corpus_root` and the duckdb CLI:
   `python3 scripts/corpus_stats.py --corpus <root> --sql "SELECT ..."`
   (views `decisions`, `events`, `outcomes` are pre-created; header rows
   already filtered). `--outcomes` gives the canned result tally and
   works without duckdb too. Look at outcomes to weight rules that
   correlate with winning, and at `events` rejections to spot confused
   play.
3. **Respect the existing library.** Do not re-propose approved chips
   unless your evidence contradicts them (then say so in the rationale —
   the merge will surface it as a conflict for the developer). Never
   resurrect a rejected chip without addressing its recorded
   `reject_reason` in your rationale.
4. **Write `proposals.json`.** Cite real evidence moments (episode +
   tick from the records; the validator rejects invented ones). Propose
   new predicates when the vocabulary cannot express a distinction you
   can see in the data — the `go_draft` is a starting point for the
   developer, not something that runs as-is.
5. **Validate.**
   `python3 scripts/validate_proposals.py --request analysis-request.json --proposals proposals.json --out validated-proposals.json`
   Exit 0 means clean; exit 1 lists every dropped or corrected claim.
   Iterate until the report is clean or the remaining issues are ones
   you can justify to the reviewer (e.g. a deliberate conflict).
6. **Hand off.** Tell the user to merge and review:
   `go run ./cmd/behavior-merge -library chips.json -request analysis-request.json -proposals validated-proposals.json`
   then approve in `go run ./cmd/behavior-editor -library chips.json`.
   The merge re-validates everything independently — your validated file
   is a convenience, not a bypass.

## Judgement guidance

- A rule with high coverage and many counterexamples is a bad rule, not
  a good rule with noise — the counterexamples are the interesting
  data. Look at those records and ask what predicate would separate
  them; that is usually your best `predicates` proposal.
- Fewer, cleaner, well-ordered rules beat many overlapping ones: the
  generated code is a decision list a human will read.
- The corpus is the spec. If the corpus never shows a situation, say so
  in `notes` instead of guessing a rule for it.
