---
id: concept:hybrid-synchronization
type: concept
title: Hybrid Synchronization
---
Default scheme: clients send actions upward, server sends deltas, events, and snapshots downward.

```yaml
upstream: data:player-input
downstream: data:state-delta, data:game-event, data:snapshot
combines: concept:input-synchronization and concept:state-synchronization
exchange: flow:hybrid-sync-exchange
```
