---
id: sample:tron
type: sample
title: Tron Light Cycle
---
Real-time simulation scaled from two participants to many, with mixed controllers, spectators, and departures.

```yaml
players: 2 to 8 competitive
new_capability: many simultaneous participants, and what happens when the set of participants changes
timing: real time, simultaneous input
synchronization: world state oriented, growing trails make data:state-delta clearly cheaper than snapshots
visibility: global scope
exercises:
  - mixed human and bot fields in one match
  - permission:spectator-receive-only, including the dedicated ack need of concept:ack-transmission-policy
  - concept:agent-departure-policy and concept:agent-proxy-designation on a real disconnect
  - per receiver baseline cost of decision:framework-side-delta-generation at eight receivers
  - concept:training-farm load testing with bot only fields
role: the step between one on one realtime and world projection
```
