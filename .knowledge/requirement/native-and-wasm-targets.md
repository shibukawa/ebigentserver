---
id: requirement:native-and-wasm-targets
type: requirement
title: Native And WebAssembly Targets
---
Client builds must support both native and WebAssembly (js/wasm) with the same game code; servers are native-only.

```yaml
constraint: wasm restricts available transports to browser-reachable ones
browser_transports:
  - system:websocket
  - system:webtransport
  - system:webrtc
excluded_in_browser: system:quic-udp raw sockets
decision: decision:webtransport-primary-for-wasm
wasm_roles: concept:standalone-mode and concept:listen-server-mode only
wasm_listen_works_because: concept:static-host-mode rendezvous needs no listening port
no_wasm_dedicated: a dedicated server binds a listener, which a browser cannot
dropped_targets:
  wasip1: go wasip1 has no net.Listen, so a wasip1 server was never runnable; as an engine-free canary it duplicated what importcheck proves directly (dropped 2026-08-21)
  linux_386: dropped 2026-08-21; 32-bit determinism is fixmath's own concern, not a per-project build matrix row
ci_matrix: js/wasm only, excluding the host toolchain (cli, cmd/ebigent)
```
