---
id: system:webtransport
type: system
title: WebTransport
---
HTTP/3 and QUIC based transport reachable from browsers.

```yaml
capabilities: reliable streams and unreliable datagrams
role: primary browser transport for concept:dedicated-server-mode
requires: http/3 endpoint and valid certificate, separate from system:edge-tls-terminator
interface: api:transport-interface
```
