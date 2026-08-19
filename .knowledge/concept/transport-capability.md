---
id: concept:transport-capability
type: concept
title: Transport Capability
---
Capability set a transport offers, used instead of protocol name when selecting transport.

```yaml
capabilities:
  - reliable ordered stream
  - unreliable datagram
  - peer to peer connectivity
  - browser reachability
measurements: rtt and loss estimate surfaced by api:sequence-ack-layer, consumed by concept:delta-baseline-policy
rule: rule:transport-selected-by-capability
interface: api:transport-interface
```
