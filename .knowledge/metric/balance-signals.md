---
id: metric:balance-signals
type: metric
title: Balance Signals
---
Aggregate measures extracted from large episode volumes to find game balance problems.

```yaml
signals:
  - win rate by agent profile
  - death cause distribution
  - movement path clustering
  - item selection frequency
  - tactic frequency
  - cooperative action rate
  - unwinnable or stuck state occurrence
source: data:episode-log corpora produced by concept:simulation-farm
computation: sql aggregates over the decisions, events, and outcomes streams, typically with system:duckdb
ground_truth_allowed: these are outcome measures, so the world stream may be read here, unlike condition synthesis under rule:analysis-restricted-to-visible-fields
flow: flow:automated-playtest
```
