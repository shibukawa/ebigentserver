---
id: decision:xvfb-for-coupled-game-servers
type: decision
title: Virtual Display For Coupled Game Servers
---
A server for a tier b or tier c game of concept:engine-coupling-tier runs against a virtual display, because linking system:ebitengine forces a display connection on linux.

```yaml
decided: yes
forced_by:
  fact: on linux the engine initializes glfw in package init, not in RunGame
  consequence: an engine linked binary panics with no DISPLAY and fails to load at all without libX11, whether or not RunGame is called
  measured: ebitengine v2.9.10, panic at internal/ui/ui.go:101 with libX11 present and DISPLAY unset; loader error in a bare debian slim image
  not_linux: windows and macos have no equivalent barrier, so a developer machine runs the same artifact windowless
tier_b_runtime: virtual display plus the x11 client library only; no gl stack is needed because nothing renders
tier_c_runtime: virtual display plus software rendering, roughly 250 mb of libraries and continuous draw work
verified:
  tier_b: input reads outside RunGame return zero values instead of failing, image creation and draw calls do not crash, and a self paced loop held 300 ticks at 60 hz
  tier_c: full RunGame held tps pacing at 60 and fast forwarded at tps 600, so concept:game-time-control speeds survive a virtual display
why_acceptable: the cost lands on deployment of an already coupled game, not on the framework, and never on tier a
tier_a_unaffected: a separated game still ships the pure go artifact with no display of any kind, which stays the recommended form
rejected_alternative: requiring every game to reach tier a before it can host a server
rejection_reason: it excludes the majority of existing Ebitengine games, which is the population this framework wants to serve
```
