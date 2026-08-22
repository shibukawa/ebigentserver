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
entry: api:run-wrapper, which wraps the run with options call
absent_in: concept:simulation-mode, and concept:dedicated-server-mode at tier a of concept:engine-coupling-tier
linked_but_unused: a tier b or tier c server links the engine and still hosts, see decision:xvfb-for-coupled-game-servers
linking_is_not_free:
  linux: glfw initializes in package init, so importing the engine demands a display connection before main runs, whatever the process intends to do
  other_platforms: no equivalent barrier, so an engine linked binary runs windowless on a developer machine
typical_game_shape: logic embedded in the game interface implementation, scenes swapped by swapping implementations; the framework accommodates this rather than requiring separation first
headless_facility: exists as an experimental host and guest pair, see decision:vm-host-not-for-servers
```
