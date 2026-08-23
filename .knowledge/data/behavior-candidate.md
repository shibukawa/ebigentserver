---
id: data:behavior-candidate
type: data
title: Behavior Candidate
---
One proposed behavior rule produced by log analysis, carrying the evidence and reasoning that justify it.

```yaml
fields:
  - condition: the situation predicate, stated over concept:sight fields only
  - action: the concept:action or subtree to run
  - priority: where it sits among sibling candidates
  - rationale: why the analyzer believes this condition selects this action
  - evidence: concept:behavior-evidence references into data:episode-log
  - coverage: how many observed decisions this rule would reproduce
  - conflict: existing nodes it overlaps or contradicts
  - proposed_levels: which skill levels should enable it, see concept:skill-level-gating
status: candidate until a developer accepts it, see rule:generated-behavior-requires-approval
constraint: a condition referencing state outside concept:sight is invalid, since the agent cannot read it at runtime
```
