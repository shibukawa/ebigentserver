---
id: requirement:offline-simulation
type: requirement
title: Offline Simulation
---
Sessions must run without rendering, input devices, or realtime clock.

```yaml
mode: concept:simulation-mode
clock: concept:game-time-control
motivation: slow agents such as actor:llm-agent need non-realtime pacing
```
