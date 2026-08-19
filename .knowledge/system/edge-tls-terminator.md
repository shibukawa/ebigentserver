---
id: system:edge-tls-terminator
type: system
title: Edge TLS Terminator
---
CDN or load balancer terminating HTTPS in front of the HTTP control plane.

```yaml
fronts: system:popcorn-wave
not_in_path_of: realtime transports such as system:webtransport, system:webrtc, system:quic-udp
implication: realtime endpoints need their own certificate and port strategy
```
