---
id: concept:training-farm
type: concept
title: Training Farm
---
Parallel execution of many sessions with differently profiled agents.

```yaml
agent_mix: beginner, expert, aggressive, defensive, cooperative, exploratory via concept:behavior-profile
runs: concept:training-mode sessions with every concept:player-slot filled by an agent, see decision:no-ai-game-mode
feeds: concept:behavior-distillation, which is why the corpus is the point and not the play
produces: data:episode-log at volume
analysis: metric:balance-signals
unattended_variant: concept:continuous-match-loop, which never finishes and rotates pairings instead
```
