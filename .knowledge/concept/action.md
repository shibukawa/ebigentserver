---
id: concept:action
type: concept
title: Action
---
Game-defined intent emitted by concept:agent and consumed by concept:session.

```yaml
property: identical action type regardless of producing agent kind
producers: human input via api:input-adapter, bot policy, remote peer, replay stream
wire_form: data:player-input
not: raw device input, see rule:no-engine-input-in-game-logic
```
