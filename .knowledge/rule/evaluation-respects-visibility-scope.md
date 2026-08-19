---
id: rule:evaluation-respects-visibility-scope
type: rule
title: Evaluation Respects Visibility Scope
---
A data:evaluation-signal delivered to a slot must be computable from what that slot can see.

```yaml
hazard: an evaluation derived from full concept:world-state leaks hidden state as a number
example: the value drops because an unseen enemy approached, so the agent infers a threat it was never shown
options:
  - scoped: compute from the same projection the slot received, see concept:visibility-scope
  - privileged: compute from ground truth, but then mark it and withhold it from agents during play
analysis_use: a privileged signal is still valid for labelling outcomes offline, like the world stream under rule:analysis-restricted-to-visible-fields
consistency: the scope choice is declared in data:visibility-annotation with the rest of the projection
```
