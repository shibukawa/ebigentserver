---
id: requirement:native-and-wasm-targets
type: requirement
title: Native And WebAssembly Targets
---
Client builds must support both native and WebAssembly with the same game code.

```yaml
constraint: wasm restricts available transports to browser-reachable ones
browser_transports:
  - system:websocket
  - system:webtransport
  - system:webrtc
excluded_in_browser: system:quic-udp raw sockets
decision: decision:webtransport-primary-for-wasm
```
