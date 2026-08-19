---
id: sample:pong
type: sample
title: Pong
---
Minimal real-time sample: continuous input, server tick, authoritative physics, snapshots.

```yaml
players: 2 competitive
new_capability: the realtime loop; everything before this was request and response
timing: real time, fixed tick rate from data:session-tuning-profile
synchronization: world state oriented, see concept:state-synchronization
visibility: global scope
exercises:
  - flow:agent-decision-loop at rate
  - data:player-input upstream, data:snapshot and data:state-delta downstream
  - decision:fixed-point-numeric-representation in physics
  - client interpolation and latency handling
  - api:sequence-ack-layer and concept:delta-baseline-policy under real loss
smallest_because: two paddles and a ball are the least state that still needs authoritative simulation
deliberately_absent: many players, partial visibility, disconnect handling
```
