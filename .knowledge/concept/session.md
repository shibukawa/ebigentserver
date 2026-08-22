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
scope: one match, built by api:roster when gathering ends rather than at process start, see concept:match-lifecycle
tick_commit: rule:deterministic-tick-commit
game_seams_during_play: api:tick-hooks
operational_bounds: data:runtime-resource-budget
independence: rule:session-independent-of-transport-and-agent-kind
placement: concept:execution-topology
```
