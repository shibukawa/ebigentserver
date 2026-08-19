---
id: concept:state-synchronization
type: concept
title: State Synchronization
---
Authoritative session distributes world state to clients.

```yaml
payload: data:state-delta, data:snapshot
content: positions, velocities, hp, entity lifecycle, status
requires: term:server-authority
fits: concept:dedicated-server-mode, concept:listen-server-mode
```
