---
id: concept:delta-baseline-policy
type: concept
title: Delta Baseline Policy
---
Tunable choice of which retained snapshot a data:state-delta is computed against, trading bandwidth against loss recovery.

```yaml
axis: how speculative the sender is about receipt
modes:
  - name: confirmed_only
    baseline: newest version acked by api:sequence-ack-layer
    property: safe; every delta is decodable on arrival
    cost: delta grows with rtt and loss because the baseline lags
  - name: speculative
    baseline: last sent version
    property: minimum bandwidth per message
    cost: one lost message invalidates the following chain until resync
  - name: bounded_speculation
    baseline: last sent, up to n unconfirmed versions, then fall back to confirmed
    property: bounded worst case, tunable per game
  - name: adaptive
    baseline: chosen per receiver from measured loss and rtt
    property: realtime games speculate while lossy links stay safe
fallback: send full data:snapshot when the confirmed baseline is older than retained history
applies_to: data:state-delta and concept:agent-view updates alike
selection: per session configuration, and may differ per receiver
```
