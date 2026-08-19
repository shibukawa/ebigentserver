---
id: rule:regeneration-preserves-approved-nodes
type: rule
title: Regeneration Preserves Approved Nodes
---
Re-running analysis on more play data produces a diff against data:behavior-tree, never a replacement.

```yaml
protects: hand authored nodes, hand edited conditions, and level gates set by a developer
diff_classes:
  - new candidate, not previously seen
  - candidate matching a previously rejected one, shown as such with the old reason
  - contradiction with an approved node, which needs an explicit decision
  - approved node now weakly supported by evidence, a signal that the game changed
never: silent overwrite of an approved node
surface: diff view in ui:behavior-tree-editor
```
