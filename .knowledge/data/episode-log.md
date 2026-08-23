---
id: data:episode-log
type: data
title: Episode Log
---
Recorded concept:episode data, written as JSONL streams that analysis tools read directly.

```yaml
format: one json object per line, utf8, append only, crash safe at line granularity
streams:
  - decisions: data:decision-record, the primary analysis table
  - events: data:game-event occurrences with tick and cause
  - outcomes: one row per episode, see metric:episode-outcome
  - world: optional full concept:world-state ground truth for debugging, never joined into behavior analysis by default
stream_separation_reason: each file keeps a stable column set, which columnar readers handle far better than one mixed record type
header_row: schema version, data:game-version, rng seed of rule:shared-rng-seed, evaluation_version of data:evaluation-signal
recording_mode: concept:episode-recording-mode, declared in the header
compression: gzip or zstd, read directly by system:duckdb without a decompression step
two_representations:
  runtime: may record compactly, including sight deltas from concept:agent-view, to survive 60hz recording
  analysis: materialized JSONL, and converted to parquet once a corpus is queried repeatedly
volume_control: analysis_sampled may record decision points and sample long sessions; replay_complete may not omit required records
integrity: replay_complete stores periodic data:state-checkpoint
not_recorded: data:presence-message, see rule:presence-excluded-from-simulation-and-analysis
governance: policy:episode-data-governance
consumers: actor:replay-agent, actor:rl-agent, flow:behavior-tree-synthesis, flow:behavior-profile-derivation
```
