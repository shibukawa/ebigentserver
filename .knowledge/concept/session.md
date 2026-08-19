---
id: concept:session
type: concept
title: Session
---
Runtime that advances game state by applying agent actions on each tick.

```yaml
holds:
  - agent set
  - tick clock, see term:tick
  - simulation step
  - pending actions
  - concept:world-state
  - emitted events
  - snapshot capability
  - replay recording, see concept:episode
lifecycle: concept:session-lifecycle
tick_commit: rule:deterministic-tick-commit
operational_bounds: data:runtime-resource-budget
independence: rule:session-independent-of-transport-and-agent-kind
placement: concept:execution-topology
```
