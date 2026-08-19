---
id: rule:presence-superseded-not-retransmitted
type: rule
title: Presence Is Superseded Not Retransmitted
---
data:presence-message is sent unreliably and never repaired, because a newer sample makes a lost one worthless.

```yaml
channel: unreliable and unordered, see concept:transport-capability
no_ack: api:sequence-ack-layer tracks nothing for this channel
no_baseline: not diffed, so concept:delta-baseline-policy and rule:delta-baseline-must-be-retained do not apply
queue_behavior: one bounded slot per sender, overwritten rather than appended, so a stalled link never accumulates stale cursors
congestion_order: first traffic class dropped under the backpressure of api:message-framing, ahead of data:player-input and data:state-delta
stale_handling: receivers may fade or hide a presence indicator that stops updating, a display concern rather than a protocol one
consequence: presence costs bandwidth only when there is bandwidth to spare
```
