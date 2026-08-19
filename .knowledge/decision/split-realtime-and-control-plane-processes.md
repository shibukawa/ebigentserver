---
id: decision:split-realtime-and-control-plane-processes
type: decision
title: Split Realtime And Control Plane Processes
---
Production runs the control plane and the realtime game server as separate processes on separate network paths.

```yaml
decided: yes
control_plane_path: client to https to cdn or alb to http to system:popcorn-wave
realtime_path: client to system:webtransport, system:webrtc, or system:quic-udp to game process
rationale: system:popcorn-wave does not terminate tls and assumes an edge in front, see system:edge-tls-terminator
game_process_contents: realtime endpoint, concept:session, tick loop, simulation, rollback
game_process_contract: requirement:production-runtime-safety and api:runtime-observability
dev_exception: decision:combined-local-dev-process
```
