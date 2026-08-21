---
id: concept:deployment-combination
type: concept
title: Deployment Combination
---
Which combinations of seat count, host, transport, and signaling actually reach each other, and which are only arithmetically possible.

```yaml
axes:
  seats: 1, 2, or many — concept:participant-shape
  host: none in process, a playing process (concept:listen-server-mode), or a non playing one (concept:dedicated-server-mode)
  link: pipe, system:webrtc, system:webtransport, system:websocket, system:quic-udp
  signaling: concept:signaling-method
governing_constraint: >
  the host decides which links are reachable, not the seat count. A dedicated
  host has a name and a certificate, so browsers reach it over webtransport
  with websocket as the fallback. A playing host has neither and usually sits
  behind nat, so it is reached over webrtc, over a link local address, or not
  at all. This is why seats and host are separate questions.
combinations:
  - name: solo
    seats: 1
    host: in process
    link: pipe
    signaling: none
    trust: the standalone case of concept:trust-model
  - name: local_pair
    seats: 2
    host: playing
    link: pipe
    signaling: none
    use: development and tests, both agents in one process
  - name: lan_pair_or_party
    seats: 2 or many
    host: playing
    link: system:webtransport or system:quic-udp on a link local address
    signaling: api:lan-discovery
    limit: native only — a browser cannot send udp broadcast
    trust: the direct_pair case of concept:trust-model
  - name: invited_pair_or_party
    seats: 2 or many
    host: playing
    link: system:webrtc
    signaling: api:manual-signaling-token
    use: concept:static-host-mode — static files, no backend of any kind
    trust: the invited_pair case of concept:trust-model
  - name: brokered_peer
    seats: 2 or many
    host: playing
    link: system:webrtc
    signaling: a concept:control-plane ticket naming the peer
    trust: the brokered_pair case of concept:trust-model
  - name: central
    seats: 2 or many
    host: dedicated
    link: system:webtransport, falling back to system:websocket, see decision:webtransport-primary-for-wasm
    signaling: a concept:control-plane ticket naming the server
    trust: the central_session case of concept:trust-model, the only one whose results deserve ranked stakes
excluded:
  - name: mesh
    why: peer links are star shaped here, so many peers means one host and n links, never n squared, see concept:static-host-mode
  - name: dedicated over webrtc
    why: arithmetically fine and pointless — a host that already has a certificate and a routable name gains nothing from nat traversal
  - name: playing host over webtransport on the open internet
    why: needs a certificate and an inbound port the player does not have; the same shape works on a lan, which is why it appears there instead
seat_count_effects: slot set, admission capacity, and whether concept:agent-departure-policy has to keep a session alive; not which transport is used
```
