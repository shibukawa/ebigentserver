---
id: rule:generated-agent-code-is-deterministic
type: rule
title: Generated Agent Code Is Deterministic
---
Generated predicates and behavior code obey the same numeric rules as the simulation whenever the agent is simulated rather than transmitted.

```yaml
two_cases:
  - name: agent_as_input_source
    situation: the agent runs on one machine and its concept:action is transmitted or recorded
    requirement: none beyond correctness; the action is data by the time others see it
  - name: agent_simulated_everywhere
    situation: bots run inside the simulation on every peer, or are resimulated during term:rollback
    requirement: full term:determinism, since a differing decision diverges the world
consequence: generated predicates use api:fixed-point-math, never float distance or angle math
enforced_by: rule:codegen-rejects-nondeterministic-types applied to generated agent code as well as to concept:world-state
cost_bound: predicates run per agent per tick, so generation must reject unbounded scans over all entities
```
