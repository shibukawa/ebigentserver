---
id: rule:identity-token-not-accepted-by-session
type: rule
title: Identity Token Not Accepted By Session
---
concept:session accepts data:session-ticket only, never data:identity-token.

```yaml
rationale:
  - an identity token has no session scope, so it cannot express seat, role, or expiry per match
  - it is long lived, so leaking it to a player run listen server is a lasting compromise
  - the game process would need the identity provider audience and key set, breaking requirement:control-plane-decoupling
consequence: exchange happens in concept:control-plane during flow:session-admission
```
