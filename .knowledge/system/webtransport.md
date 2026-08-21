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
direction: client to server only — one end must be a listening server with a name and a certificate
does_not_replace_webrtc:
  - a browser cannot listen, so a browser-hosted session is unreachable over this transport at all; that is concept:static-host-mode, and it is why system:webrtc exists here
  - there is no nat traversal, so two players on separate home networks cannot reach each other without a server in the middle
  - a self signed certificate is accepted only by pinning its hash, under limits short enough to suit a lan rather than the open internet
replaces_webrtc_when: a dedicated host is present in every deployment, which is the central row of concept:deployment-combination and the case where dedicated over webrtc is already excluded
implementation: native only, //go:build !js && !wasip1; a browser build reaches the platform api instead
interface: api:transport-interface
```
