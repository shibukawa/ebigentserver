---
id: concept:lag-compensation
type: concept
title: Lag Compensation
---
Server-side rewind that judges a client action against the world as that client saw it.

```yaml
mechanism: on receiving an action, restore the world version the client had at that tick, evaluate, then reapply
requires: term:server-authority and retained world history
history_source: the versions already retained for decision:framework-side-delta-generation, so the cost is largely shared
applicable_when: an authoritative session judges instantaneous interactions, for example hitscan
not_applicable_when:
  - term:rollback already resimulates from inputs, making this redundant
  - turn based and strategy play, where the delay is inside the decision anyway
  - concept:state-synchronization without instantaneous judgement
cost_of_enabling: the shooter advantage moves to the shooter; a target can be hit after moving behind cover
tuning: lag_compensation_window of data:session-tuning-profile, zero disables it
genre_dependence: this is a game design choice about fairness, not a correctness feature
```
