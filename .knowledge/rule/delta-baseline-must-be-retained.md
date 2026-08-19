---
id: rule:delta-baseline-must-be-retained
type: rule
title: Delta Baseline Must Be Retained And Named
---
Every data:state-delta names the version it was computed from, and the sender retains that version until it is acked or abandoned.

```yaml
sender_obligation: keep every baseline still referenced by an unconfirmed delta
receiver_obligation: reject a delta whose named baseline it does not hold, and request resync
resync: full data:snapshot, see concept:delta-baseline-policy fallback
memory_cost: retained versions equal receiver count times speculation depth of concept:delta-baseline-policy
bound: exceeding the retention budget forces a snapshot rather than an unbounded buffer
```
