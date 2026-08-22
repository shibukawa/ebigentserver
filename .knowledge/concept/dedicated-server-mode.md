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
runs: only the arbitrate hook of api:tick-hooks; intake and apply belong to clients
gathering: no screen, so api:roster fills from the listener alone and the process returns to idle for the next match, see concept:match-lifecycle
no_rendering_means: no draw calls, which is not the same as no engine linked; what a server links depends on concept:engine-coupling-tier
native_only: a browser cannot bind a listener, so this mode has no wasm form, see requirement:native-and-wasm-targets
```
