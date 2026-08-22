---
id: api:simulation-interface
type: api
title: Simulation Interface
---
Contract a game implements so concept:session can host its rules.

```yaml
type_parameters:
  S: concept:world-state
  A: concept:action
  O: concept:observation
operations:
  - name: start
    input: the seed of rule:shared-rng-seed
    output: initial S
  - name: acting_slots
    output: the concept:player-slot set that must decide this step
    empty_means: no further decisions, legal only when every slot evaluates terminal
  - name: apply
    input: one concept:action already accepted by api:action-validator
    constraint: must not fail, since legality was settled before the call
  - name: project
    output: one slot's concept:observation, see rule:observation-content-owned-by-game
  - name: evaluate
    output: data:evaluation-signal, see rule:evaluation-computed-by-session
  - name: advance
    scope: realtime pacing only, one simulation step after the tick's inputs land
counterpart: api:agent-interface, the other half of flow:agent-decision-loop
naming: decision:simulation-not-game
constraints:
  - rule:no-float-in-simulation
  - rule:deterministic-simulation-required-for-rollback, when the topology needs it
```
