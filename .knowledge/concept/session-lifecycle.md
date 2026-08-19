---
id: concept:session-lifecycle
type: concept
title: Session Lifecycle
---
State machine governing admission, play, bounded shutdown, and terminal outcomes of one concept:session.

```yaml
states:
  - created: configuration validated; no agents admitted
  - admitting: pre start agents may join; simulation not committed
  - running: actions and ticks accepted; game policy may admit returning or late agents
  - draining: new admission and actions rejected; final outputs flush until deadline
  - ended: normal terminal state
  - aborted: unrecoverable failure terminal state
transitions:
  - created -> admitting: initialization succeeds
  - admitting -> running: game start condition succeeds
  - running -> draining: game end, operator stop, or process shutdown
  - draining -> ended: final data:snapshot, data:episode-log, and lifecycle callbacks finish
  - draining -> aborted: drain deadline expires before required final output finishes
  - created|admitting|running|draining -> aborted: invariant violation or unrecoverable runtime failure
rules:
  - terminal transition occurs once
  - admission is forbidden in created and draining
  - agent join completes before first observe
  - agent leave completes before session end callback
  - draining deadline comes from data:runtime-resource-budget
  - process crash is aborted; transparent live migration is not a base framework guarantee
```
