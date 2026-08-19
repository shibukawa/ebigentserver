---
id: system:tinybind
type: system
title: Tinybind
---
Existing Go code generation and binding library shared by the framework and the control plane.

```yaml
existing_features: struct analysis, json binding, config binding, cli binding
added_features:
  - cbor generation for concept:cbor-wire-profile and concept:cbor-world-profile
  - scale aware fixed point encode and decode, see decision:fixed-point-numeric-representation
  - diff and patch generation for decision:framework-side-delta-generation
  - determinism validation, see rule:codegen-rejects-nondeterministic-types
  - data:protocol-version derived from the generated schema
position: shared base under both this framework and system:popcorn-wave
decision: decision:reuse-tinybind-codegen
```
