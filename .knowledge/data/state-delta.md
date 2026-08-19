---
id: data:state-delta
type: data
title: State Delta Message
---
Incremental concept:world-state change sent from an authoritative session to clients.

```yaml
content: changed entity fields, entity creation, entity deletion
header:
  - sequence number from api:sequence-ack-layer
  - baseline version id, see rule:delta-baseline-must-be-retained
  - target tick, see term:tick
baseline_choice: concept:delta-baseline-policy
produced_by: framework diffing, see decision:framework-side-delta-generation
encoding: concept:cbor-wire-profile or concept:cbor-world-profile by size and stability
used_by: concept:state-synchronization
```
