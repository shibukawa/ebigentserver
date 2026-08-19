---
id: system:ebitengine
type: system
title: Ebitengine
---
Go 2D game engine used as the first-class client for rendering and human input.

```yaml
targets: native and webassembly
provides: game loop, rendering, keyboard, mouse, gamepad input
boundary: api:input-adapter, see rule:no-engine-input-in-game-logic
absent_in: concept:dedicated-server-mode, concept:simulation-mode
```
