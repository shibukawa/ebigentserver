---
id: decision:separate-game-cli
type: decision
title: Separate Game CLI From PW CLI
---
Ship api:game-cli as its own toolchain rather than extending the system:popcornweb pw command.

```yaml
decided: yes
rationale: build targets, simulation, replay, and training have no control plane counterpart
shared_layer: system:tinybind codegen and binding features
serves: requirement:dedicated-game-toolchain
```
