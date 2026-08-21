---
id: api:dev-debug-endpoint
type: api
title: Dev Debug Endpoint
---
Local inspection channel a development build opens for ui:dev-console to attach to.

```yaml
address: loopback by default, set in data:run-config; absent when unset
linkage: present only in a development entry point, see rule:debug-endpoint-excluded-from-release
streams:
  - tick: per tick duration against budget, missed ticks, send rate, snapshot and delta bytes
  - state: current concept:world-state plus the concept:agent-view derived for each concept:agent
  - decision: data:decision-record as each agent produces it
  - lifecycle: concept:session-lifecycle transitions, admission rejects, and disconnect reasons
encoding: reuses concept:cbor-world-profile for state, so the console decodes the same bytes the session holds
direction: read mostly; the only accepted writes are concept:game-time-control commands, being pause, step, and time_scale
authority_boundary: never accepts an concept:action and never edits state; a paused session resumes on the same tick under rule:deterministic-tick-commit
relation: a development superset of api:runtime-observability, not a replacement for it
```
