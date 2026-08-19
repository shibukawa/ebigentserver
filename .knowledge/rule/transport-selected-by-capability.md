---
id: rule:transport-selected-by-capability
type: rule
title: Transport Selected By Capability
---
Code selects transports by concept:transport-capability, never by protocol name.

```yaml
example: unreliable datagram need is satisfied by system:webtransport or system:quic-udp
fallback: reliable ordered only, satisfied by system:websocket
benefit: browser and native builds share selection logic
```
