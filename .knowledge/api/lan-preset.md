---
id: api:lan-preset
type: api
title: LAN Preset
---
Composed host and guest calls that let two instances of one binary find each other and play, with no server to run and no address to type.

```yaml
composes: api:lan-discovery for finding, flow:session-admission for seating, api:transport-interface over system:websocket for carrying, concept:state-synchronization for the downstream
game_supplies: data:game-version, the generated codec of concept:cbor-world-profile, an input encoder pair, and the projection; this preset encodes nothing itself
host_operations:
  - open: listen, mint from a key generated at startup, and announce
  - attach: install the downstream hook into the session config before concept:session exists
  - serve: wire the finalized match and admit everyone who was waiting
guest_operations:
  - browse: passive, so it can only find hosts on the segment
  - join: take a seat, then wait inside the handshake until the host starts
ordering_constraint: >
  a peer can only be admitted once its seat has an inbox, which means
  once the session exists, but people arrive while api:roster is still
  gathering. So a guest connection is accepted early and parked, and the
  wait is the lobby rather than a poll.
control_plane_is_the_host: the seat grant and the ticket come from the host process, since on a segment there is nothing else to ask, see decision:control-plane-features-out-of-scope
trust: the direct_pair case of concept:trust-model; scope is the control, so the host refuses to listen off rule:unauthenticated-admission-requires-scope-or-capability's ranges
remote_player_is_an_agent: >
  the host seats a detached slot and reads its inbox, so the authoritative
  side never learns whether a person or a bot is across the link. The real
  agent runs in the guest process, which is why replacing one with the
  other is a seating decision, see decision:agent-as-central-abstraction
unavailable_in: browser builds, which cannot send udp broadcast
```
