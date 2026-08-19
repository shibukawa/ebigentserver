---
id: system:webrtc
type: system
title: WebRTC
---
Peer capable transport usable from browsers and native clients.

```yaml
capabilities: reliable and unreliable data channels, peer to peer connectivity
channel_plan:
  - ordered reliable: data:snapshot, control messages, admission
  - unordered unreliable: data:player-input, data:state-delta, see concept:transport-capability
fits: one to one, small player counts, term:rollback, fighting games, concept:static-host-mode
framing: per message size limits require api:message-framing for large payloads
overhead: signaling, ice, nat traversal
signaling: concept:signaling-method, either concept:control-plane or api:manual-signaling-token
identity_binding: dtls fingerprint, used by rule:ticket-bound-to-connection
interface: api:transport-interface
```
