---
id: flow:peer-authentication
type: flow
title: Peer Authentication
---
Two peers matched by a central service verify each other before a p2p session starts.

```yaml
flow:
  trigger: concept:control-plane matches two authenticated players
  steps:
    - id: issue_pair
      actor: concept:control-plane
      output: one data:session-ticket per peer, each naming the same session_id and the expected peer set
    - id: signal
      actor: concept:control-plane
      action: relay system:webrtc signaling, binding each peer transport fingerprint into its ticket
    - id: exchange
      action: peers present tickets to each other over the established channel
    - id: verify_mutual
      action: each peer verifies signature, expiry, session_id, and that the presenter is in its own peer set
    - id: verify_binding
      action: compare the ticket fingerprint claim against the actual transport fingerprint, see rule:ticket-bound-to-connection
    - id: start
      actor: concept:session
      action: the hosting peer seats both agents
  failure:
    session_id_mismatch: a ticket from another match is rejected, so tickets cannot be pooled or traded
    fingerprint_mismatch: a copied ticket presented from another machine is rejected
limit: this authenticates identity, not honesty; see the brokered_pair case of concept:trust-model
```
