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
called_from: the intake hook of api:tick-hooks
device_set: limited to the devices declared through api:run-wrapper, which also bounds how a player joins in ui:lobby-scene
on_a_server: never called, and any device read left inside server driven code returns zero values rather than failing
```
