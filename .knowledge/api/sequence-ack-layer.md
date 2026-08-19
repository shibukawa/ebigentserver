---
id: api:sequence-ack-layer
type: api
title: Sequence And Ack Layer
---
Transport frontend that numbers outgoing messages, reports which ones it received, and detects silence.

```yaml
position: below concept:session, above the raw protocol of api:transport-interface
mechanism:
  - assign a sequence number to every outgoing message
  - record the highest received sequence plus a bitfield of recent receipts
  - deliver that ack record by concept:ack-transmission-policy
  - bidirectional, both ends run the same layer
exposes_to_session:
  - confirmed_baseline: newest state version the peer is known to hold
  - inflight: versions sent but not yet confirmed
  - loss_estimate and rtt, used by concept:delta-baseline-policy
liveness:
  realtime_case: continuous tick traffic is itself the liveness signal; a silence deadline measured in missed ticks needs no extra messages
  idle_case: turn based play, spectators, and slow agents under decision:dual-mode-agent-pacing produce no traffic, so an explicit probe is required
  disconnect: declare loss after the silence deadline, plus a short grace period when the transport reports a recoverable state
  after_loss: hand the seat to concept:agent-departure-policy; the layer keeps no resume state, see decision:no-mid-session-reconnect
applies_to: unreliable datagram channels
reliable_channel_case: delivery is already guaranteed, so last sent equals eventual baseline; the layer still reports application level acceptance
```
