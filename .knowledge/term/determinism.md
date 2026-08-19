---
id: term:determinism
type: term
title: Deterministic Simulation
---
Same initial state plus same action sequence yields identical state on every machine and every run.

```yaml
threats: floating point variance, map iteration order, unseeded randomness, wall clock reads
countermeasures:
  - decision:fixed-point-numeric-representation removes float variance
  - rule:shared-rng-seed removes randomness variance
  - rule:deterministic-tick-commit removes arrival and scheduling variance
  - deterministic iteration over concept:world-state structs
verification: compare data:state-checkpoint at equal ticks
required_by: term:rollback, concept:input-synchronization, actor:replay-agent
rule: rule:deterministic-simulation-required-for-rollback
```
