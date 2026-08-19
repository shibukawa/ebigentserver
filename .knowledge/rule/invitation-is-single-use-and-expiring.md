---
id: rule:invitation-is-single-use-and-expiring
type: rule
title: Invitation Is Single Use And Expiring
---
An api:manual-signaling-token is bearer data, so its safety comes from being short-lived and redeemable once.

```yaml
required_properties:
  - short lifetime, minutes not hours
  - one participant per invitation; a second presenter is rejected
  - not enumerable, since it is a random capability rather than an address
  - stripped from the visible url after being read
residual_exposure: browser history, clipboard, and any chat service the link passed through
not_provided: identity; whoever holds the link is the participant, see the invited_pair case of concept:trust-model
upgrade_path: issue data:session-ticket from concept:control-plane when identity actually matters
```
