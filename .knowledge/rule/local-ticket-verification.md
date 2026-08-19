---
id: rule:local-ticket-verification
type: rule
title: Local Ticket Verification
---
The verifier checks a data:session-ticket signature locally without calling the issuer.

```yaml
rationale: the connection path must not depend on concept:control-plane availability or latency
verifier: a game process, or a peer machine in the p2p cases of concept:trust-model
requires: the issuer public key set, short ticket expiry, see rule:asymmetric-ticket-signature
accepted_cost: public key distribution and rotation are an operational responsibility, not framework scope
rotation_hint: accept multiple active key ids so rotation does not drop live sessions
unknown_kid: refresh keys before readiness; admission still fails closed
replay_defense: jti redemption is local to one session, see rule:ticket-bound-to-connection
clock_skew: keep expiry short but allow bounded skew, since verifiers include player machines
operations: policy:protocol-rollout, api:runtime-observability
flow: flow:session-admission
```
