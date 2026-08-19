---
id: decision:framework-side-delta-generation
type: decision
title: Framework Side Delta Generation
---
Framework retains snapshots and computes data:state-delta by diffing them; games do not hand write delta code.

```yaml
decided: yes
mechanism: keep retained versions per receiver, diff against current concept:world-state
generated_by: system:tinybind struct analysis over decision:go-struct-world-state types
game_responsibility: declare struct types and field scales, not diff logic
baseline_selection: concept:delta-baseline-policy
receipt_tracking: api:sequence-ack-layer
cost: retained versions scale with receiver count times speculation depth, bounded by rule:delta-baseline-must-be-retained
same_mechanism_serves: concept:agent-view incremental update
```
