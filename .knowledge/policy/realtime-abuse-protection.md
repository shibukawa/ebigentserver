---
id: policy:realtime-abuse-protection
type: policy
title: Realtime Abuse Protection
---
Realtime endpoints validate and bound untrusted traffic before it consumes session resources.

```yaml
admission:
  - rate limit by endpoint and available connection identity
  - validate framing, protocol version, ticket, role, and seat before allocation
  - unknown signing kid, invalid claim, and exhausted capacity fail closed
decode:
  - enforce data:runtime-resource-budget before allocation
  - reject malformed CBOR, excessive nesting, oversized collections, and decompression expansion
runtime:
  - enforce per connection input rate and action count
  - repeated violations close the connection with a stable reason code
transport:
  - untrusted networks require authenticated encryption
  - websocket uses tls; system:webtransport, system:webrtc, and quic retain their secure handshake
  - raw udp requires application authenticated encryption or an explicitly trusted network scope
scope: applies even when identity authentication is omitted by decision:no-auth-on-lan
evidence: api:runtime-observability without logging bearer credentials
```
