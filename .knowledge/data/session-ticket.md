---
id: data:session-ticket
type: data
title: Session Ticket
---
Short-lived signed credential that admits one client to one session.

```yaml
encoding: jwt, see decision:jwt-session-ticket
standard_claims:
  - sub: player id
  - aud: the session endpoint that may accept it
  - exp: short expiry, seconds to a couple of minutes
  - jti: unique id, redeemable once
  - kid: key id, so keys rotate without dropping live sessions
game_claims:
  - session_id
  - role: player, spectator, or observer, see permission:spectator-receive-only
  - seat: which agent slot, see permission:agent-seat-control
  - peers: expected participant set, required for peer verified topologies
  - endpoint
binding: rule:ticket-bound-to-connection
verification: rule:local-ticket-verification with rule:asymmetric-ticket-signature
issuer: concept:control-plane
flow: flow:session-admission
```
