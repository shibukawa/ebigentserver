---
id: system:quic-udp
type: system
title: QUIC And UDP Transport
---
Native transport for clients that can open raw sockets.

```yaml
capabilities: unreliable datagram, quic streams
security:
  quic: authenticated encrypted handshake
  raw_udp: application authenticated encryption or explicitly trusted network scope, see policy:realtime-abuse-protection
unavailable_in: browser builds, see requirement:native-and-wasm-targets
fits: native client to concept:dedicated-server-mode
interface: api:transport-interface
```
