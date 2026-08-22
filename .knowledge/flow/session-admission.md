---
id: flow:session-admission
type: flow
title: Session Admission
---
Client joins a game session using a short-lived signed ticket instead of a control plane call.

```yaml
flow:
  trigger: player requests a match
  steps:
    - id: authenticate
      actor: concept:control-plane
      action: authenticate the player, producing data:identity-token
    - id: match
      actor: concept:control-plane
      action: run lobby and matchmaking, then allocate a session
    - id: issue
      actor: concept:control-plane
      output: signed data:session-ticket with endpoint, seat, and role
    - id: connect
      actor: actor:remote-agent
      action: open a transport connection and present the ticket in the first message
    - id: bound
      actor: api:transport-interface
      action: apply policy:realtime-abuse-protection and data:runtime-resource-budget before session allocation
    - id: version_check
      actor: concept:session
      action: compare data:game-version, see rule:game-version-must-match
    - id: verify
      actor: concept:session
      action: validate signature and claims locally, see rule:local-ticket-verification
    - id: bind
      actor: concept:session
      action: redeem the jti and bind identity to the connection, see rule:ticket-bound-to-connection
    - id: seat
      actor: concept:session
      action: bind the connection to an agent slot and apply permission:agent-seat-control
    - id: seed
      actor: concept:session
      output: initial data:snapshot carrying the seed of rule:shared-rng-seed
  failure:
    version_mismatch: reject with an explicit protocol version error
    invalid_or_expired_ticket: reject connection without contacting the control plane
    replayed_jti: reject, the seat is already occupied by the original holder
    capacity_or_rate_limit: reject with a stable reason and api:runtime-observability evidence
p2p_variant: flow:peer-authentication
reentry: a returning player runs this same flow with a new ticket, see decision:no-mid-session-reconnect
```
