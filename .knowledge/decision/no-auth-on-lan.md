---
id: decision:no-auth-on-lan
type: decision
title: No Authentication On LAN
---
Direct one-to-one and LAN play run without authentication; no invite code mechanism is built.

```yaml
decided: yes
applies_to: the direct_pair and invited_pair cases of concept:trust-model
rejected_alternatives: pre shared invite codes, self signed peer identities, a local issuer
rationale:
  - the network boundary, or the invitation capability, already limits who can reach the endpoint
  - a local issuer would duplicate concept:control-plane for the case with the least to protect
  - identity has no meaning without a persistent account, which is control plane scope
consequence: any reachable client may join, so reachability becomes the control, see rule:unauthenticated-admission-requires-scope-or-capability
unchanged: policy:observation-scoped-information still applies, so a joining peer sees only its own concept:observation
```
