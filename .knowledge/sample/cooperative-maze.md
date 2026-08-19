---
id: sample:cooperative-maze
type: sample
title: Cooperative Maze
---
Team play where humans and AI fill complementary roles and no participant sees the whole world.

```yaml
players: 2 to 4 cooperative
new_capability: partial visibility plus role differentiation, and AI as a teammate rather than an opponent
roles:
  - scout: wider sight range
  - engineer: opens special doors
  - carrier: transports objectives
  - navigator: knows destination information
timing: real time in concept:sample-acceptance-matrix; turn based is an optional non coverage variant
synchronization: world state oriented with per slot projection
visibility: self and team scopes of concept:visibility-scope
exercises:
  - concept:agent-view producing genuinely different projections per slot
  - rule:observation-content-owned-by-game, since roles decide content
  - actor:llm-agent constrained to concept:action only, a natural fit for role play
  - filling empty roles with AI, proving decision:no-ai-game-mode for cooperation
llm_value: the corpus of cooperative episodes feeds flow:behavior-tree-synthesis with teamwork behavior
```
