---
id: system:popcorn-wave
type: system
title: Popcorn Wave
---
Separate web and API framework usable as a concept:control-plane implementation.

```yaml
typical_use: authentication, lobby, matchmaking, ranking api, profile api, session allocation
tls: does not terminate tls, runs behind system:edge-tls-terminator
relation: optional peer, not a dependency, see decision:independent-from-popcorn-wave
process_split: decision:split-realtime-and-control-plane-processes
```
