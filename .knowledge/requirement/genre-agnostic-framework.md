---
id: requirement:genre-agnostic-framework
type: requirement
title: Genre Agnostic Framework
---
Framework must not assume a game genre, rule set, or world model.

```yaml
scope: concept:session, concept:agent, concept:world-state stay opaque to game rules
game_supplies: simulation step, action set, sight projection, serialization types
framework_supplies: tick loop, transport, synchronization, replay, agent hosting
serves: vision:agent-session-runtime
```
