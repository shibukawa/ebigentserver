---
id: data:state-checkpoint
type: data
title: State Checkpoint
---
Deterministic digest proving that independent executions reached the same committed state.

```yaml
fields:
  - tick
  - data:protocol-version
  - canonical_world_hash
  - rng_position
  - accepted_action_hash
canonical_input: concept:cbor-world-profile encoding with stable field and collection order
emitted: periodically, at replay verification boundaries, and on desync diagnosis
stored_in: data:snapshot, data:episode-log
failure: unequal digest at equal tick and version is a deterministic simulation error
```
