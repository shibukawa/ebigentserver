---
id: decision:go-struct-world-state
type: decision
title: Go Struct World State
---
concept:world-state is plain Go structs, not a generic component store or dynamic map.

```yaml
decided: yes
rejected_alternatives: mandatory ecs, reflection based dynamic property bag
rationale:
  - game code stays ordinary Go
  - system:tinybind can analyze the same structs it already handles
  - static layout makes framework side diffing possible
consequence: game defines struct types; framework supplies traversal via generated code
simulation_type_set: types transitively reachable from the world state root, no separate annotation needed
validated_by: rule:codegen-rejects-nondeterministic-types
enables: decision:framework-side-delta-generation
```
