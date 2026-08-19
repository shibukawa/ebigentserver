---
id: decision:host-loss-ends-session
type: decision
title: Host Loss Ends The Session
---
When the process holding authority dies, the session is over; there is no migration or state handoff.

```yaml
decided: yes
applies_to: concept:listen-server-mode, concept:static-host-mode, and a crashed concept:dedicated-server-mode process
rejected_alternative: host migration by electing a peer and rebuilding from snapshots
rejection_reasons:
  - the authoritative state at the moment of loss is gone; any rebuild is a guess presented as truth
  - election plus state transfer is a distributed systems project grafted onto every game
  - concept:agent-departure-policy already covers every non host departure, which is the common case
client_behavior: report session loss and return to whatever came before, lobby or title
result_consequence: a terminal data:progress-report may never arrive; the control plane treats a session that stops reporting as abandoned, see data:progress-report
recovery: for persistent worlds the game may restart from its own saved snapshot, see decision:world-persistence-is-game-scope
```
