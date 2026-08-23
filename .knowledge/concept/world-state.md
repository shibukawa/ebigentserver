---
id: concept:world-state
type: concept
title: World State
---
Authoritative game state owned by concept:session and never exposed to agents directly.

```yaml
representation: plain Go structs, see decision:go-struct-world-state
numerics: scaled integers, see decision:fixed-point-numeric-representation
exposure: only through concept:sight, materialized as concept:agent-view
distribution_forms:
  - data:state-delta
  - data:snapshot
diffing: framework side, see decision:framework-side-delta-generation
authority: held by the session process, see term:server-authority
excludes: data:presence-message and any cosmetic value, see rule:presence-excluded-from-simulation-and-analysis
```
