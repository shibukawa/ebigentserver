---
id: concept:trust-model
type: concept
title: Trust Model
---
Who authenticates whom, who verifies, and what guarantee survives, stated per concept:execution-topology.

```yaml
cases:
  - name: standalone
    topology: concept:standalone-mode
    authentication: none
    authority: the local process
    guarantee: none needed, the player is the only participant
  - name: direct_pair
    topology: p2p one to one and lan, no issuer
    authentication: none by decision, see decision:no-auth-on-lan
    authority: whichever peer runs the authoritative concept:session
    guarantee: identity is unverified; the hosting peer can do anything
    access_control: network reachability only, enforced by rule:unauthenticated-admission-requires-scope-or-capability
    note: policy:sight-scoped-information still limits what the joining peer learns
  - name: invited_pair
    topology: concept:static-host-mode, no server of any kind
    authentication: none; the invitation is the credential
    verification: possession of an api:manual-signaling-token only
    authority: the hosting browser
    guarantee: whoever holds the link joins; senderId is a label, not proof of who someone is
    control: rule:invitation-is-single-use-and-expiring
  - name: brokered_pair
    topology: p2p play with central matchmaking
    authentication: concept:control-plane authenticates both, issues a data:session-ticket to each
    verification: mutual, each peer verifies the other, see flow:peer-authentication
    authority: the hosting peer, as in direct_pair
    guarantee: identity and pairing are verified; simulation results are still only as trustworthy as the host
  - name: central_session
    topology: concept:dedicated-server-mode, one to many
    authentication: concept:control-plane issues tickets, the server verifies each
    authority: the server, see term:server-authority
    guarantee: strongest; identity verified and simulation trusted
common_contract: data:session-ticket in every networked case, never data:identity-token
result_trust: data:progress-report from a player hosted session is forgeable; only dedicated results deserve ranked stakes
key_requirement: rule:asymmetric-ticket-signature, because in three of four cases the verifier is a player machine
```
