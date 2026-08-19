---
id: concept:ack-transmission-policy
type: concept
title: Ack Transmission Policy
---
Tunable choice of how the ack record of api:sequence-ack-layer reaches the peer.

```yaml
axis: whether ack rides on existing traffic or gets its own message
modes:
  - name: piggyback_only
    mechanism: attach the ack record to the next outgoing message
    cost: zero extra packets
    requires: a return flow at a comparable rate
  - name: dedicated
    mechanism: emit a standalone ack message
    cost: one extra packet per ack, payload is a few bytes
    use: receivers with little or no upstream traffic
  - name: delayed_piggyback
    mechanism: wait a bounded deadline for an outgoing message, then send a standalone ack
    property: piggybacks when traffic exists, degrades to dedicated when it does not
    tuning: deadline and coalescing count
real_driver: return flow rate, not bandwidth alone
cost_shape:
  bytes: negligible payload, dominated by the transport header of the datagram
  packets: packet rate is the real cost; per packet cpu and syscall cost scales with receiver count
requires_dedicated:
  - receive only participants such as spectators, whose baseline would never confirm
  - asymmetric rates, for example 60hz downstream against 20hz upstream input
  - slow upstream agents, see decision:dual-mode-agent-pacing
interaction: under concept:delta-baseline-policy confirmed_only, ack latency directly inflates delta size
selection: per session and per receiver, alongside concept:delta-baseline-policy
```
