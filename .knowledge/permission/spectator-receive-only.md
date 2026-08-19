---
id: permission:spectator-receive-only
type: permission
title: Spectator Receive Only
---
A spectator role receives concept:observation but submits no concept:action.

```yaml
granted_by: role claim of data:session-ticket
enforced_at: concept:session, all inbound actions rejected
observation_scope: usually wider than a player view, still bounded by policy:observation-scoped-information
transport_consequence: no upstream flow, so concept:ack-transmission-policy must use dedicated acks for this role
```
