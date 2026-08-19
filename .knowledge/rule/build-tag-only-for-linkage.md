---
id: rule:build-tag-only-for-linkage
type: rule
title: Build Tag Only For Linkage
---
A build tag is justified only when code cannot or must not be linked; everything else is runtime configuration.

```yaml
justified:
  - platform capability, for example api:lan-discovery using udp, absent under js and wasm
  - toolchain constraints, for example anything needing cgo, see rule:analysis-tooling-outside-game-process
  - optional heavy dependencies a game may exclude
automatic_in_go: GOOS and GOARCH constraints cover the wasm split with no custom tag
not_justified:
  - concept:game-time-control speed, which is a runtime value, not a compile time one
  - concept:execution-topology selection
  - concept:synchronization-mode, concept:delta-baseline-policy, concept:ack-transmission-policy
  - log destination, agent roster, tick rate
default: runtime configuration through data:run-config
test: if two variants could sensibly run in the same process, it is configuration
```
