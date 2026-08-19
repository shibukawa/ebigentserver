---
id: concept:dedicated-server-mode
type: concept
title: Dedicated Server Mode
---
Headless process running concept:session with no rendering or local input.

```yaml
participants: all agents connect as actor:remote-agent
transport_candidates: system:webtransport, system:webrtc, system:websocket, system:quic-udp
admission: flow:session-admission
deployment: decision:split-realtime-and-control-plane-processes
```
