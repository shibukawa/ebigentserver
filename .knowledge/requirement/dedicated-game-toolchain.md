---
id: requirement:dedicated-game-toolchain
type: requirement
title: Dedicated Game Toolchain
---
Framework ships its own CLI separate from the control plane CLI.

```yaml
cli: api:game-cli
targets: concept:build-target
reuse: system:tinybind for json binding, config binding, cbor generation, cli binding, struct analysis
decision: decision:separate-game-cli
```
