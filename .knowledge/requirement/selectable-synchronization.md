---
id: requirement:selectable-synchronization
type: requirement
title: Selectable Synchronization
---
Synchronization strategy must be chosen independently of network topology.

```yaml
strategies: concept:synchronization-mode
topologies: concept:execution-topology
anti_pattern: hardcoding p2p to term:rollback or dedicated server to server authority
decision: decision:separate-topology-from-synchronization
```
