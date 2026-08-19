---
id: concept:simulation-mode
type: concept
title: Simulation Mode
---
Headless session run without rendering or realtime pacing, used for AI and testing.

```yaml
agents: actor:llm-agent, actor:behavior-tree-agent, actor:script-bot-agent, actor:replay-agent
clock: concept:game-time-control
outputs: data:episode-log
composition: a headless concept:build-target plus a time_scale and agent roster in data:run-config; the training mode is that pair, not a separate build
serves: requirement:offline-simulation, requirement:ai-autoplay-testing
```
