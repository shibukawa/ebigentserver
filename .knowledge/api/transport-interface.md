---
id: api:transport-interface
type: api
title: Transport Interface
---
Abstraction that carries agent and session messages over any supported protocol.

```yaml
operations:
  - name: connect
    input: endpoint, data:protocol-version, data:session-ticket
    cancellation: caller deadline
  - name: send_reliable
    channel: ordered stream
    errors: closed, cancelled, too_large, backpressure
  - name: send_unreliable
    channel: datagram, may drop or reorder
    errors: closed, cancelled, too_large, backpressure
  - name: receive
    cancellation: caller deadline or close
  - name: close
    idempotent: yes
concurrency:
  - connect and close serialize lifecycle transitions
  - concurrent sends preserve order only within one reliable channel
  - receive returns one ownership safe message at a time
retry: framework never retries accepted concept:action; reconnect is fresh flow:session-admission
frontend: api:sequence-ack-layer sits between this interface and concept:session
declares: concept:transport-capability
implementations: local in-process, system:websocket, system:webtransport, system:webrtc, system:quic-udp
fallback: system:websocket when datagram transports are unavailable
security: policy:realtime-abuse-protection
```
