---
id: rule:shared-rng-seed
type: rule
title: Shared RNG Seed
---
All participants of one concept:session share a single RNG seed exchanged at session start.

```yaml
exchange_point: join handshake of flow:session-admission
carrier: initial data:snapshot or join message
scope: one seed per session, advanced only by the simulation step
forbidden: local unseeded randomness, wall clock derived randomness
serves: term:determinism, term:rollback, actor:replay-agent
replay: seed is stored in data:episode-log so a run reproduces exactly
```
