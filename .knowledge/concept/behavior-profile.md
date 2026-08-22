---
id: concept:behavior-profile
type: concept
title: Behavior Profile
---
Continuous personality vector describing how an agent plays, replacing fixed difficulty classes.

```yaml
axes:
  - aggression
  - caution
  - teamwork
  - exploration
  - reaction_delay
  - prediction_accuracy
  - risk_tolerance
expresses: beginner, expert, aggressive, defensive, supportive, explorer as points not classes
axis_role: execution quality and disposition, paired with the knowledge axis of concept:skill-level-gating
derived_by: flow:behavior-profile-derivation
consumed_by: actor:behavior-tree-agent, concept:training-farm
```
