---
id: rule:no-float-in-simulation
type: rule
title: No Float In Simulation Path
---
Simulation code uses api:fixed-point-math only; float and the Go math package are confined to presentation.

```yaml
banned_in: concept:world-state, simulation step, concept:action, all message payloads
allowed_in: rendering, interpolation, camera, particles, audio, ui
hazards_avoided:
  - go spec permits fusing multiply and add into one operation, so arm64 and amd64 differ
  - math.Exp on amd64 selects an fma path from cpu features, so results differ between cpus of one architecture
  - trig and exp are not specified by ieee 754, so implementations differ across architectures and go versions
safe_exception: math.Sqrt is correctly rounded by ieee 754, but is still excluded to keep the boundary simple
enforcement: rule:codegen-rejects-nondeterministic-types, checked when types are generated
enforcement_property: build time error, so a float leak cannot reach production as a desync
```
