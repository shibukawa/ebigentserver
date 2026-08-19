---
id: flow:hybrid-sync-exchange
type: flow
title: Hybrid Sync Exchange
---
Default per-tick message exchange between client and authoritative session.

```yaml
flow:
  trigger: tick boundary under concept:hybrid-synchronization
  steps:
    - id: upstream
      actor: concept:agent
      output: data:player-input
    - id: simulate
      actor: concept:session
      action: apply inputs under term:server-authority
    - id: downstream_delta
      actor: concept:session
      output: data:state-delta
    - id: downstream_event
      actor: concept:session
      output: data:game-event
    - id: resync
      actor: concept:session
      output: data:snapshot on join or desync
```
