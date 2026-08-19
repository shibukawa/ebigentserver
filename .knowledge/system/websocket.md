---
id: system:websocket
type: system
title: WebSocket Transport
---
Reliable ordered transport with the widest browser and proxy compatibility.

```yaml
capabilities: reliable ordered stream only
no_datagram: head of line blocking applies to realtime traffic
role: fallback transport, see decision:webtransport-primary-for-wasm
interface: api:transport-interface
security: secure websocket with tls is required outside explicitly trusted local scope, see policy:realtime-abuse-protection
```
