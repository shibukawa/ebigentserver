---
id: concept:episode-recording-mode
type: concept
title: Episode Recording Mode
---
Recording contract separating lossless replay evidence from sampled analysis data.

```yaml
modes:
  - name: replay_complete
    requires: every delivered sight, accepted action, lifecycle transition, rng seed, protocol version, and data:state-checkpoint
    guarantee: actor:replay-agent can verify the episode without unrecorded game decisions
  - name: analysis_sampled
    allows: decision point recording, sight deltas, session sampling, and omitted world ground truth
    guarantee: suitable for aggregate analysis only; never labeled replayable
metadata: every data:episode-log declares its mode
validation: replay consumer rejects incomplete or sampled logs when replay_complete is required
```
