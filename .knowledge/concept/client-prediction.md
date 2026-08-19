---
id: concept:client-prediction
type: concept
title: Client Prediction
---
Optional client-side technique: apply own input immediately, then reconcile when the authoritative result arrives.

```yaml
status: optional per game, chosen by game feel, see decision:no-framework-tuning-defaults
applicable: term:server-authority realtime play, where waiting a round trip on own input is perceptible
not_applicable:
  - term:rollback, which already resimulates everything and subsumes it
  - turn based play, where the round trip hides inside the turn
mechanism: keep unacknowledged inputs, apply locally, and on receiving data:state-delta rewind own entity to the server result and replay the pending inputs
shares_machinery: the save and resimulate cycle of term:rollback, applied to one entity instead of the world
cost: mispredictions surface as visible corrections; games where that is worse than input delay should not enable it
tuning: enabled and bounded through data:session-tuning-profile
```
