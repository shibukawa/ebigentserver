---
id: rule:asymmetric-ticket-signature
type: rule
title: Asymmetric Ticket Signature
---
data:session-ticket is signed with a private key held only by the issuer; verifiers hold a public key.

```yaml
algorithms: eddsa or es256, never a shared symmetric secret
decisive_reason: in concept:listen-server-mode and p2p the verifier is a player machine, and a symmetric secret there is a forgery key for every session
key_distribution: public key set fetched from concept:control-plane, selected by the kid claim
rotation: accept multiple active key ids at once
offline_property: verification needs no call to the issuer, see rule:local-ticket-verification
```
