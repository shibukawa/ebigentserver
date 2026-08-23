---
id: concept:agent-view
type: concept
title: Agent View
---
Server side retained per-agent projection of concept:world-state, updated incrementally instead of rebuilt each tick.

```yaml
purpose: make concept:sight affordable at high agent counts and high tick rates
holds: last projected state per agent plus the baseline the receiver is known to hold
update: diff current world state against the retained view, emit only changes
baseline_choice: concept:delta-baseline-policy, same machinery as data:state-delta
reuses: decision:framework-side-delta-generation diffing machinery
content_ownership: rule:sight-content-owned-by-game
also_serves: term:fog-of-war, interest management by visibility filter
```
