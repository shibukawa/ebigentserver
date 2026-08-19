---
id: rule:ticket-bound-to-connection
type: rule
title: Ticket Bound To Connection
---
Identity is bound to the connection at admission; nothing later in the session may declare who it is.

```yaml
binding_steps:
  - redeem the jti once, reject reuse within the ticket lifetime
  - bind the transport fingerprint, for example the dtls certificate fingerprint of system:webrtc, so a copied ticket fails from another machine
  - map the connection to a seat, thereafter derive identity from the connection
forbidden: trusting a player id carried inside data:player-input or any in session message
protects_against: ticket replay, seat spoofing, action injection for another player
related: permission:agent-seat-control
```
