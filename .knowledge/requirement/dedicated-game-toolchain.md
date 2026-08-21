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
single_binary: requirement:unified-toolchain-binary
host_tool_not_a_target: the toolchain runs on a developer machine and is never a concept:build-target, so requirement:native-and-wasm-targets does not reach it — which is what lets it carry a terminal UI
```
