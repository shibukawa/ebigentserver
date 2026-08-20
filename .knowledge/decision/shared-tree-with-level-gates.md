---
id: decision:shared-tree-with-level-gates
type: decision
title: Shared Tree With Level Gates
---
Skill levels are expressed as gates on one shared data:behavior-tree rather than as separate trees per level.

```yaml
decided: yes
superseded_by: decision:shared-chip-library, which keeps this decision's rationale but shrinks the shared unit from the whole tree to data:behavior-chip
rejected_alternative: one tree per level
rejection_reasons:
  - a fix to shared logic must be applied n times and will drift
  - the levels stop being comparable, so a balance change cannot be reasoned about across them
  - concept:behavior-evidence would be duplicated per tree
accepted_costs:
  - one tree grows more complex than any single level needs
  - a node edit can affect several levels at once, so the editor must show which levels a change touches
escape_hatch: a level specific subtree is still expressible, since gates are tag sets, see concept:skill-level-gating
```
