---
id: data:curated-corpus
type: data
title: Curated Corpus
---
Output of the curate stage: a filtered, deduplicated, split copy of a recorded corpus, ready for distillation.

```yaml
layout:
  - train/<episode>/decisions.jsonl: rows kept for mining, headers copied verbatim from data:episode-log
  - holdout/<episode>/decisions.jsonl: episodes reserved for evaluation, never mined
  - report.json: counts read, kept, dropped; unique situations; duplication top list; conflicts with per-action counts
  - gaps.jsonl: holdout situations no approved chip answers, the target list for the next recording round; written by the distill entry's holdout evaluation (behavior.Evaluate), since curate has no vocabulary or library
provenance: report.json records source root, filters, cap, holdout ratio, and seed, so a curated corpus reproduces from its inputs
consumers: the curate step of flow:behavior-tree-synthesis; ebigent distill points --corpus at train
origin: decision:curate-corpus-to-corpus, serving requirement:corpus-curation
```
