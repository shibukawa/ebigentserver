---
id: rule:deterministic-simulation-required-for-rollback
type: rule
title: Determinism Required For Input Synchronization
---
Games choosing term:rollback or concept:input-synchronization must keep the simulation deterministic.

```yaml
requires: term:determinism
provided_by:
  - decision:fixed-point-numeric-representation
  - rule:shared-rng-seed
  - rule:deterministic-tick-commit
verified_by: data:state-checkpoint
also_required_by: actor:replay-agent reproducing state from actions
not_required_by: concept:state-synchronization under term:server-authority
```
