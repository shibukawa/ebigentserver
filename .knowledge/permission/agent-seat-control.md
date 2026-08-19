---
id: permission:agent-seat-control
type: permission
title: Agent Seat Control
---
A seated connection may submit concept:action only for its own agent slot.

```yaml
granted_by: seat claim of data:session-ticket, naming a concept:player-slot, fixed at admission
enforced_at: concept:session, before an action enters the simulation
check: action seat equals connection seat, derived by rule:ticket-bound-to-connection
violation: drop the action and flag the connection, never apply it
note: this is authorization; legality and plausibility are the next stage, see api:action-validator
```
