---
id: concept:build-target
type: concept
title: Build Target
---
Named artifact kind produced from one game codebase, realized as one entry point.

```yaml
mechanism: decision:entry-points-over-build-tags
matrix:
  - target: native client
    renders: yes
    links: system:ebitengine, all transports, api:lan-discovery
  - target: wasm client
    renders: yes
    links: system:ebitengine, system:websocket, system:webtransport, system:webrtc
    excludes: udp sockets, so api:lan-discovery is unavailable
  - target: static wasm bundle
    renders: yes
    links: system:webrtc and api:manual-signaling-token only
    mode: concept:static-host-mode
  - target: listen server
    renders: yes
    note: a client that also hosts, so it links everything a client and a session need
  - target: dedicated server
    renders: no
    links: transports and session only at tier a of concept:engine-coupling-tier, where system:ebitengine is never imported; a tier b or tier c project links it and simply never runs the engine loop
    mode: concept:dedicated-server-mode
    native_only: yes
  - target: simulation
    renders: no
    links: session and local transport only
    mode: concept:training-mode
shared_by_all: the game rules package, which links into every target unchanged
entry_naming: the playable entry carries the game's own name, since it is the binary a developer runs and hands to somebody; a headless server is the same directory under a build tag rather than a directory of its own
one_main_function: every target starts at api:run-wrapper, which branches on the topology in data:run-config, so an untagged artifact already hosts and the tag exists for display free deployment
built_by: api:game-cli
constraint: requirement:native-and-wasm-targets
```
