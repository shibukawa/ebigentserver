---
id: requirement:corpus-curation
type: requirement
title: Corpus Curation
---
A curate stage must run between recording and distillation: a raw human corpus is formally valid input yet does not distill into a good bot.

```yaml
motivation:
  one_record_one_vote: behavior.SequentialCovering counts every data:decision-record equally; 100 repeats of one board add 100 coverage, with no dedup or frequency correction
  knobs_are_bounds: MinCoverage, MaxRules, MaxEvidence cap the output; they repair no corpus duplication or bias
  merge_scope: behavior.Merge reconciles data:behavior-chip proposals, never the decision rows behind them
raw_corpus_failure_modes:
  - coverage_gap: situations absent from the corpus leave the bot inactive or misorder its rules
  - duplication_bias: frequent easy situations inflate coverage and push rare important decisions down the list
  - policy_mixing: different human choices in one situation become counterexamples under the deterministic decision list, not personality or variance
stages:
  - filter: select rows by agent_kind, player, data:game-version, outcome
    note: behavior.Segment's keep func sees the whole data:decision-record row (widened from seat-only), so agent_kind separation works at both curate and segment time
  - aggregate: group identical situations, keep per-action counts
  - cap: bound how many rows one frequent situation contributes
  - prioritize: keep rare situations, counterexamples, and outcome-split decisions preferentially
  - split: divide into training and validation sets at concept:episode granularity
  - gap_detection: report uncovered situations to target in the next play or simulation round
position: before the segment step of flow:behavior-tree-synthesis, feeding concept:behavior-distillation
design: decision:curate-corpus-to-corpus, producing data:curated-corpus
```
