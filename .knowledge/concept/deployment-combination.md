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
  - name: local_loopback
    seats: 2 or many
    host: playing
    link: transport/pipe, an in process Conn pair
    signaling: none
    distinct_from_solo: solo admits agents straight into the concept:session; this one puts a transport between them, so encoding, snapshot and delta generation, and api:sequence-ack-layer all run without a network
    also_a_fault_rig: the pipe injects loss, latency, jitter, and reordering per direction, which is how term:rollback, concept:lag-compensation, and concept:delta-baseline-policy recovery get tested against a bad link that does not have to exist
    reproducibility: the drop and reorder pattern is seeded and repeats; arrival timing rides real timers and does not, which is deliberate
    use: decision:combined-local-dev-process, and what the dev verb of api:game-cli runs
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
    server_role: control path only — it matches, authenticates, and signals, then leaves the data alone
    trust: the brokered_pair case of concept:trust-model
  - name: central
    seats: 2 or many
    host: dedicated
    link: system:webtransport, falling back to system:websocket, see decision:webtransport-primary-for-wasm
    signaling: a concept:control-plane ticket naming the server
    server_role: data path — every exchange between two players is two hops instead of one
    trust: the central_session case of concept:trust-model, the only one whose results deserve ranked stakes
  - name: relayed_peer
    seats: 2 or many
    host: playing
    link: system:webrtc through a turn relay
    server_role: data path, but only as a fallback — the peers tried to reach each other and could not
    use: the escape hatch a peer to peer game needs for restrictive networks, already noted as optional in concept:static-host-mode
    cost: the relay hop plus relay bandwidth, paid only by the sessions that need it
excluded:
  - name: mesh
    why: peer links are star shaped here, so many peers means one host and n links, never n squared, see concept:static-host-mode
  - name: dedicated over webrtc as a primary path
    why: a host with a certificate and a routable name gains nothing from nat traversal; as a fallback for peers that cannot reach each other it is the relayed_peer row above, which is a different thing
  - name: playing host over webtransport on the open internet
    why: needs a certificate and an inbound port the player does not have; the same shape works on a lan, which is why it appears there instead
seat_count_effects: slot set, admission capacity, and whether concept:agent-departure-policy has to keep a session alive; not which transport is used
data_path_or_control_path: >
  the sharpest question once a link exists is whether the server carries
  the traffic or only arranges it. On the data path it sees everything and
  can be trusted with the simulation, at the cost of a second hop on every
  exchange between two players. On the control path only, the peers reach
  each other directly and the hop disappears, but the authority moves onto
  a player machine, so results become forgeable — concept:trust-model says
  as much. concept:realtime-intensity decides whether that hop is
  affordable, and it is the twitch tier where it usually is not.
p2p_data_path_commits_to_determinism: >
  peers that exchange only actions have to reproduce the same world from
  them, which is concept:input-synchronization and therefore
  rule:deterministic-simulation-required-for-rollback. Avoiding the relay
  hop is not free: it is paid for in the simulation being reproducible bit
  for bit, which constrains every rule the game will ever write.
transport_pair_is_not_redundant: >
  system:webtransport covers every row with a dedicated host and system:webrtc
  covers every row with a playing host, and neither reaches the other's rows.
  A project that will always ship a server needs only the first; a project
  that wants the no-backend path needs the second, because no other browser
  api lets one browser accept a connection from another.
```
