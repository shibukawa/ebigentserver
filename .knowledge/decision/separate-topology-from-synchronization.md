---
id: decision:separate-topology-from-synchronization
type: decision
title: Separate Topology From Synchronization
---
concept:execution-topology and concept:synchronization-mode are chosen independently.

```yaml
decided: yes
rejected_assumption: p2p implies term:rollback, dedicated server implies server authority
consequences:
  - p2p with authority and dedicated server with rollback both remain expressible
  - transport choice does not fix netcode choice
serves: requirement:selectable-synchronization
```
