---
id: data:behavior-chip
type: data
title: Behavior Chip
---
Small situation-scoped behavior unit — one condition over predicate vocabulary paired with one action or small subtree — kept in a shared library and composed into agents.

```yaml
origin: an accepted data:behavior-candidate graduates into a chip; hand-authored chips are equally valid
fields:
  - condition: over data:derived-predicate only, observation-limited per rule:analysis-restricted-to-visible-fields
  - action: a concept:action, or a small subtree of further chips
  - priority_hint: default ordering among siblings, overridable per data:agent-loadout
  - tags: open tag dimensions; level of concept:skill-level-gating, style, and tactic of concept:tactic-selector are all the same mechanism
  - evidence: concept:behavior-evidence retained from distillation
  - tests: rule:predicate-tests-generated-from-episodes
storage: one shared library per game, see decision:shared-chip-library; a chip is never duplicated into an agent
granularity: one situation-to-action judgement, small enough to name, review, and benchmark alone
example: fighting game — pressure_rush when opponent_is_turtling; patient_approach when projectile_incoming
consequence: fixing a chip fixes every data:agent-loadout that selects it, which is the anti-drift point
```
