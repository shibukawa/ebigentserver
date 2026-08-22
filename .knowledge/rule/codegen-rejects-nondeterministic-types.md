---
id: rule:codegen-rejects-nondeterministic-types
type: rule
title: Codegen Rejects Nondeterministic Types
---
Code generation fails when a simulation struct contains a field that cannot behave deterministically.

```yaml
check_point: system:tinybind struct analysis, before emitting encode, decode, and diff code
scope: types reachable from the concept:world-state root, see decision:go-struct-world-state
rejected:
  - name: float32 and float64
    reason: platform variance and fused multiply add, see rule:no-float-in-simulation
  - name: map
    reason: Go randomizes map iteration order, so traversal and diff output vary per run
  - name: interface and pointer to shared value
    reason: identity and aliasing are not reproducible from a snapshot
  - name: time.Time and wall clock derived values
    reason: not a function of term:tick
  - name: fixed point field with no declared scale
    reason: scale is part of data:game-version and cannot be inferred
map_alternative: ordered slice, or generated traversal in sorted key order
failure_mode: build error naming the type and field, not a runtime warning
enforces: rule:no-float-in-simulation, term:determinism
```
