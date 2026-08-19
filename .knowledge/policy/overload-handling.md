---
id: policy:overload-handling
type: policy
title: Overload Handling
---
Deterministic degradation order used when data:runtime-resource-budget is reached.

```yaml
order:
  - supersede data:presence-message
  - replace queued obsolete state deltas with a newer baseline valid update
  - pause reliable producers at the configured high water mark
  - disconnect a persistently slow or violating receiver
  - reject new admission when process capacity is exhausted
never_drop_silently: accepted concept:action, lifecycle control, terminal result, replay_complete data:episode-log
session_failure: abort through concept:session-lifecycle when simulation cannot meet its tick or memory invariant
recording_failure: abort replay_complete recording and mark the artifact incomplete rather than mislabeling it replayable
requirements:
  - every shed, reject, disconnect, and abort emits api:runtime-observability evidence
  - overload behavior is independent of transport implementation
```
