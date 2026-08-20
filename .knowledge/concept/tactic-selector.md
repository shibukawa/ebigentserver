---
id: concept:tactic-selector
type: concept
title: Tactic Selector
---
Root selection layer that switches the active chip group mid-game by situation, so one agent changes strategy without re-instantiation.

```yaml
model:
  - the compiled tree's root selects among the tactic groups of the data:agent-loadout
  - tactic conditions are data:derived-predicate over concept:observation, so switching is deterministic and replays exactly
  - static code: tactics compile with the tree per decision:behavior-tree-compiled-to-go; nothing is rebuilt at runtime
player_orders: a tactic order to an allied agent is an ordinary concept:action — it enters concept:world-state, is projected into concept:observation, and the selector reads it there, so decision:no-ai-game-mode holds
example: rush, zoning, and turtle groups in a fighting game; squad orders in a tactics game
constraint: tag evaluation of concept:skill-level-gating stays at instantiation; tactic switching is in-tree branching, not gate re-evaluation
recorded: the active tactic at each decision is visible in data:decision-record via the observation, so ui:chip-benchmark can chart tactic frequency
```
