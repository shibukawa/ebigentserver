---
id: flow:automated-playtest
type: flow
title: Automated Playtest
---
Run a fleet of profiled agents to test the game itself, not only the AI.

```yaml
flow:
  trigger: build change or balance change
  steps:
    - id: configure
      action: define agent mix by concept:behavior-profile
    - id: run
      actor: concept:simulation-farm
      action: execute many concept:simulation-mode sessions in parallel
    - id: aggregate
      action: collect data:episode-log results
    - id: report
      output: metric:balance-signals, metric:episode-outcome
    - id: regress
      action: replay stored episodes with actor:replay-agent to detect behavior change
serves: requirement:ai-autoplay-testing
```
