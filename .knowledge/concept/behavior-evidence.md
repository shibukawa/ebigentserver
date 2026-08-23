---
id: concept:behavior-evidence
type: concept
title: Behavior Evidence
---
The link from a proposed rule back to the recorded situations that produced it.

```yaml
purpose: a developer cannot judge a condition in the abstract; they judge the situations it fires in
holds:
  - episode id and tick range in data:episode-log
  - the concept:sight at the decision point
  - the concept:action actually taken
  - the outcome that followed, see metric:episode-outcome
uses:
  - jump to that moment with actor:replay-agent
  - show counterexamples: situations matching the condition where a different action was taken
counterexamples_matter: a rule with high coverage and frequent counterexamples is a bad rule, and only the evidence view reveals that
```
