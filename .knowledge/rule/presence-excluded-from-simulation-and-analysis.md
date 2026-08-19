---
id: rule:presence-excluded-from-simulation-and-analysis
type: rule
title: Presence Excluded From Simulation And Analysis
---
data:presence-message enters neither the simulation nor the behavior analysis corpus.

```yaml
excluded_from_simulation:
  reason: arrival timing is arbitrary and values are ui space, so admitting it would break term:determinism
  effect: no rollback, no replay dependency, no protocol version pressure from cosmetic fields
excluded_from_analysis:
  storage: not written to the decisions stream of data:episode-log
  reason: a runtime actor:behavior-tree-agent must not condition on cursor jitter, and the volume would dominate the corpus
  accepted_cost: some human decisions were influenced by a teammate cursor, so a few outliers become unexplainable to flow:behavior-tree-synthesis
  mitigation_if_ever_needed: record presence to a separate stream, never joined by default
excluded_from_replay: actor:replay-agent reproduces play without it; presence is atmosphere, not history
```
