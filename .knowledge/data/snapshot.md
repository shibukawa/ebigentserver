---
id: data:snapshot
type: data
title: Snapshot
---
Full concept:world-state capture used for join, resync, rollback, replay seeding, and delta baselines.

```yaml
encoding: concept:cbor-world-profile
triggers: agent join, desync detection, periodic checkpoint
carries_on_join: rng seed of rule:shared-rng-seed
integrity: carries data:state-checkpoint for its committed tick
retained: per receiver as the diff baseline of decision:framework-side-delta-generation
used_by: term:rollback restore, actor:replay-agent
```
