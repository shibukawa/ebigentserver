---
id: system:popcornweb
type: system
title: Popcorn Web
---
Separate web and API framework usable as a concept:control-plane implementation.

```yaml
former_name: popcorn wave, renamed before release; older notes may still use it
binary: pw
typical_use: authentication, lobby, matchmaking, ranking api, profile api, session allocation
tls: does not terminate tls, runs behind system:edge-tls-terminator
relation: optional peer, not a dependency, see decision:independent-from-popcornweb
process_split: decision:split-realtime-and-control-plane-processes
```
