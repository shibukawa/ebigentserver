---
id: decision:shared-chip-library
type: decision
title: Shared Chip Library
---
Agents are assembled from one shared data:behavior-chip library; the shared unit is the chip, not a whole tree.

```yaml
decided: yes
generalizes: decision:shared-tree-with-level-gates, keeping its anti-drift rationale with a smaller shared unit
model:
  - all agents of a game draw from one data:behavior-chip library
  - one agent personality is a data:agent-loadout, a selection over the library plus a concept:behavior-profile vector
  - level tags of concept:skill-level-gating become one tag dimension among several: level, style, tactic
preserved_rationale:
  - a shared chip is fixed once and every loadout that selects it benefits; no per-agent drift
  - loadouts stay comparable because they are selections over identical units, so ui:chip-benchmark deltas are attributable
  - concept:behavior-evidence lives on the chip, never duplicated
new_capability: many distinct CPU personalities from one library, plus mid-game strategy change via concept:tactic-selector
compilation: a loadout materializes as one data:behavior-tree and compiles to Go via decision:behavior-tree-compiled-to-go
```
