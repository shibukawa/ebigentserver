---
id: api:input-adapter
type: api
title: Input Adapter
---
Game-supplied mapping from engine device state to concept:action.

```yaml
input: system:ebitengine keyboard, mouse, gamepad state
output: concept:action for the current term:tick
scope: per game, per control scheme
rule: rule:no-engine-input-in-game-logic
flow: flow:input-adaptation
```
