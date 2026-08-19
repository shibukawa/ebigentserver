---
id: concept:execution-topology
type: concept
title: Execution Topology
---
Placement of concept:session and concept:agent across processes and the transport between them.

```yaml
options:
  - local, see concept:standalone-mode
  - listen server, see concept:listen-server-mode
  - dedicated server, see concept:dedicated-server-mode
  - p2p
orthogonal_to: concept:synchronization-mode
decision: decision:separate-topology-from-synchronization
```
