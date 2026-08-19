---
id: metric:episode-outcome
type: metric
title: Episode Outcome
---
Per-episode result and reward used for comparison and learning.

```yaml
fields:
  - result, the terminal field of data:evaluation-signal
  - reward, accumulated from recorded reward_delta values
  - duration in ticks, see term:tick
  - agent profile reference, see concept:behavior-profile
consumers: actor:rl-agent, flow:behavior-profile-derivation
```
