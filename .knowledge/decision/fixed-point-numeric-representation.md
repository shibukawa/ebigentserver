---
id: decision:fixed-point-numeric-representation
type: decision
title: Fixed Point Numeric Representation
---
Simulation and wire numerics use scaled integers instead of floating point.

```yaml
decided: yes
rationale:
  - float results vary across platforms and break term:determinism
  - scaled integers encode as compact cbor integers, no float tag needed
  - one rule serves both determinism and payload size
scope: concept:world-state fields, data:player-input, data:state-delta, data:snapshot
representation: value times a per-field scale factor, stored as sized integer
math_library: api:fixed-point-math, core recommended from system:fixpointcs
codegen: system:tinybind emits scale aware encode and decode
float_use: presentation and rendering only, see rule:no-float-in-simulation
rule: rule:fixed-point-on-wire
```
