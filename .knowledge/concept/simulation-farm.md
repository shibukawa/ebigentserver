---
id: concept:simulation-farm
type: concept
title: Simulation Farm
---
Parallel execution of many sessions with differently profiled agents.

```yaml
agent_mix: beginner, expert, aggressive, defensive, cooperative, exploratory via concept:behavior-profile
runs: concept:simulation-mode sessions with every concept:player-slot filled by an agent, see decision:no-ai-game-mode
produces: data:episode-log at volume
analysis: metric:balance-signals
unattended_variant: concept:continuous-match-loop, which never finishes and rotates pairings instead
```
