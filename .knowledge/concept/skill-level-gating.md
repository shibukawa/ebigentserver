---
id: concept:skill-level-gating
type: concept
title: Skill Level Gating
---
One data:behavior-tree expresses several skill levels by tagging each node with the levels that enable it.

```yaml
model: per node level tag set, evaluated when the agent is instantiated, not per tick
two_axes_of_difficulty:
  - knowledge: which branches exist for this level, handled here
  - execution: reaction delay and prediction accuracy, handled by concept:behavior-profile
  rationale: a beginner both fails to know the advanced option and executes the known ones worse; one axis cannot express both
not_a_monotone_subset:
  reason: real beginners do things experts never do, not merely fewer things
  consequence: a node may be tagged beginner only, so levels are tag sets rather than nested subsets
alternative_rejected: decision:shared-tree-with-level-gates records why separate trees per level were not chosen
review_surface: level matrix in ui:behavior-tree-editor
```
