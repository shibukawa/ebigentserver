---
id: data:identity-token
type: data
title: Identity Token
---
Long-lived token proving who a player is, issued and consumed by concept:control-plane only.

```yaml
audience: concept:control-plane apis such as lobby, profile, matchmaking
lifetime: hours or days, refreshable
never_sent_to: the game process, see rule:identity-token-not-accepted-by-session
exchanged_for: data:session-ticket at session allocation time
```
