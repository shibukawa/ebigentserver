---
id: ui:lobby-scene
type: ui
title: Lobby Scene
---
Default first screen of a client or listen build, which gathers players into api:roster and hands the process to the game once play starts.

```yaml
shown_during: the gathering state of concept:match-lifecycle
shows: current seats with controller kind and readiness, the invitation or discovery affordance, and how to start
joins_by: gamepad start button, mouse click, or key press, limited to the devices api:run-wrapper accepts
also_admits: remote seats arriving through flow:session-admission, so a screen and a network fill the same roster
supplies: nothing the raw api:roster does not; replacing this scene costs no capability
scene_shape: a swappable implementation of the engine game interface, matching how Ebitengine games already change scenes
default_only: a game with its own title screen calls api:roster directly and never links this
```
