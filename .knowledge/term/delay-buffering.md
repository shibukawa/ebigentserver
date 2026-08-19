---
id: term:delay-buffering
type: term
title: Input Delay
---
Delay local action application by a fixed tick count so remote actions arrive before simulation.

```yaml
cost: constant input latency
benefit: no resimulation, simpler than term:rollback
mode: option of concept:synchronization-mode
```
