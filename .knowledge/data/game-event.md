---
id: data:game-event
type: data
title: Game Event Message
---
Discrete occurrence emitted by concept:session that clients must not infer from state alone.

```yaml
examples: hit, pickup, spawn, death, objective change
encoding: concept:cbor-wire-profile
also_recorded_in: data:episode-log
promotion: the game may promote an occurrence to a data:progress-report for the control plane
```
