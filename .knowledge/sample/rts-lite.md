---
id: sample:rts-lite
type: sample
title: RTS Lite
---
Hybrid synchronization at scale: command streams upstream, large projected world state downstream.

```yaml
players: 2 to 4 competitive
new_capability: both synchronization models in one game, over a world too large to send whole
timing: real time
synchronization: concept:hybrid-synchronization, the first sample needing both directions to differ
visibility: fog of war per player, see term:fog-of-war
exercises:
  - command streams as data:player-input against a large concept:world-state
  - data:state-delta sizing pressure, where concept:delta-baseline-policy modes diverge measurably
  - concept:cbor-wire-profile and concept:cbor-world-profile side by side in one game
  - interest management through concept:agent-view, since a player sees a fraction of the map
  - multiple agents per player, so one slot issues many unit level actions
position: final step of concept:sample-progression, combining every earlier capability
```
