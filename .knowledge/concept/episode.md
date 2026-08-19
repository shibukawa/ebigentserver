---
id: concept:episode
type: concept
title: Episode
---
Record of one agent's participation in one session run.

```yaml
contains: observations, actions, events, result, reward or metrics
storage: data:episode-log
consumers: actor:replay-agent, flow:behavior-profile-derivation, metric:episode-outcome
```
