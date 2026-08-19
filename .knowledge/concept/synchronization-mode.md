---
id: concept:synchronization-mode
type: concept
title: Synchronization Mode
---
Timing and authority strategy that keeps sessions consistent, selected per game.

```yaml
options:
  - delay, see term:delay-buffering
  - rollback, see term:rollback
  - server authoritative, see term:server-authority
  - hybrid
independent_of: concept:execution-topology
data_model: concept:input-synchronization or concept:state-synchronization
constraint: rule:deterministic-simulation-required-for-rollback
input_timing: decision:input-timing-owned-by-sync-mode
client_prediction: optional companion for server authority, see concept:client-prediction
```
