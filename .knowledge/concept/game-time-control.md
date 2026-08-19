---
id: concept:game-time-control
type: concept
title: Game Time Control
---
Session clock policy decoupled from wall clock so slow agents can still participate.

```yaml
modes:
  - realtime
  - slowed, for example 0.5x or 0.1x
  - step, advanced by the agent itself
  - unlimited or accelerated
step_mode: the agent signals completion and the session advances, see decision:dual-mode-agent-pacing
motivation: actor:llm-agent may take seconds per decision
used_by: concept:simulation-mode
selection: runtime value in data:run-config, never a build tag, see rule:build-tag-only-for-linkage
realtime_exception: turn based and strategy games can host actor:llm-agent without stepping
```
