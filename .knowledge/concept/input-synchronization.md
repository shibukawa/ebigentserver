---
id: concept:input-synchronization
type: concept
title: Input Synchronization
---
Peers exchange only actions per tick and reproduce state by running the same simulation.

```yaml
payload: data:player-input
bandwidth: small, fixed size per tick
requires: term:determinism
fits: term:rollback, term:delay-buffering, p2p topology
```
