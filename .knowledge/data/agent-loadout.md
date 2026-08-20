---
id: data:agent-loadout
type: data
title: Agent Loadout
---
One AI personality: a selection of data:behavior-chip entries with priorities, grouped by tactic, plus a concept:behavior-profile vector.

```yaml
fields:
  - chips: selected library entries, with per-loadout priority overrides
  - tactics: chip groups keyed by tactic tag, consumed by concept:tactic-selector
  - profile: concept:behavior-profile vector, the execution axis
  - level: which concept:skill-level-gating tags are enabled
two_axes: chips decide what this agent knows; profile decides how well it executes — the same pairing as concept:skill-level-gating
instantiation: resolved when the agent is created; mid-game variation happens through concept:tactic-selector branching, never re-assembly
materializes_as: one data:behavior-tree per loadout, compiled per decision:behavior-tree-compiled-to-go
consumers: actor:behavior-tree-agent, concept:simulation-farm rosters, ui:chip-benchmark, concept:continuous-match-loop pairings
```
