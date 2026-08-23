---
id: concept:sight
type: concept
title: Sight
---
Per-agent projection of concept:world-state, the only world view an agent may read.

```yaml
parts:
  - self
  - visible entities
  - objective
  - data:evaluation-signal, so the agent has a criterion and not only a legal action set
  - recent events
  - environment
materialization: retained and incrementally updated as concept:agent-view
content_scope: chosen by the game, see rule:sight-content-owned-by-game
projection_rule: concept:visibility-scope
expresses:
  - fog of war, see term:fog-of-war
  - sight range
  - AI difficulty via information limits
  - cheat prevention, see policy:sight-scoped-information
human_path: rendered by system:ebitengine before a human reads it
llm_path: serialized to text in flow:llm-teacher-episode-generation
```
