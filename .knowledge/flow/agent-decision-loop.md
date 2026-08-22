---
id: flow:agent-decision-loop
type: flow
title: Agent Decision Loop
---
Per-tick cycle turning world state into the next world state through agents.

```yaml
flow:
  trigger: concept:session starts a new term:tick
  contracts: api:stage-rule-set on the game side, api:agent-interface on the controller side
  steps:
    - id: observe
      actor: concept:session
      action: project concept:world-state into per-agent concept:observation
    - id: decide
      actor: concept:agent
      action: return concept:action for the current tick
    - id: transmit
      actor: api:transport-interface
      action: deliver data:player-input when the agent is remote
    - id: validate
      actor: api:action-validator
      action: reject illegal or implausible actions before they enter the simulation
    - id: apply
      actor: concept:session
      action: order and advance collected actions by rule:deterministic-tick-commit
    - id: emit
      actor: concept:session
      output: data:game-event, data:state-delta, data:snapshot
    - id: record
      actor: concept:session
      output: data:episode-log
  failure:
    missing_input: apply synchronization policy of concept:synchronization-mode
    budget_exceeded: apply policy:overload-handling or abort through concept:session-lifecycle
```
