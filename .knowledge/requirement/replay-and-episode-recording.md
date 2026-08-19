---
id: requirement:replay-and-episode-recording
type: requirement
title: Replay And Episode Recording
---
Session must be able to record every agent observation and action as a replayable episode.

```yaml
record: concept:episode stored as data:episode-log under concept:episode-recording-mode
replay_contract: replay_complete includes every observation, accepted action, lifecycle transition, and data:state-checkpoint
replayed_by: actor:replay-agent
uses:
  - rollback verification
  - debugging
  - regression test
  - AI training
  - behavior analysis
  - balance test
```
