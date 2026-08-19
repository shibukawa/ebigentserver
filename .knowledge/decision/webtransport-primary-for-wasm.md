---
id: decision:webtransport-primary-for-wasm
type: decision
title: WebTransport Primary For WASM Dedicated Server
---
system:webtransport is the primary transport for browser clients connecting to concept:dedicated-server-mode.

```yaml
decided: yes
rationale: offers reliable streams and unreliable datagrams inside the browser
fallback: system:websocket where webtransport is unavailable
p2p_case: system:webrtc for peer topologies
serves: requirement:native-and-wasm-targets
```
