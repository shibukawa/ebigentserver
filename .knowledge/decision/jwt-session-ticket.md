---
id: decision:jwt-session-ticket
type: decision
title: JWT Encoded Session Ticket
---
data:session-ticket is a JWT rather than a custom binary credential.

```yaml
decided: yes
rationale:
  - every control plane language already has a signing and verification library
  - claims, expiry, audience, and key id are standardized, so nothing is invented
  - it appears once at connection time, so size does not matter, unlike concept:cbor-wire-profile traffic
  - json at the control plane boundary matches rule:profile-selection-by-message-kind
signature: asymmetric only, see rule:asymmetric-ticket-signature
not_used_for: in session messages, which stay binary
```
