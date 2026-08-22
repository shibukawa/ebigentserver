---
id: api:lan-discovery
type: api
title: LAN Discovery
---
Broadcast beacon that lets native clients find sessions on the same network without any server.

```yaml
mechanism: periodic udp broadcast or multicast beacon from the host, passive listen on the client
beacon_payload: game package path, data:game-version, title, endpoint, player count, whether a data:session-ticket is required
game_identity: >
  the package path and data:game-version compared as one pair, see
  decision:module-path-is-game-identity. It is what separates one game's
  beacons from another's, since every ebigent build broadcasts on one port;
  the title beside it is for display and identifies nothing.
client_action: list responding sessions, then connect through api:transport-interface
availability: native builds only; browsers cannot send udp broadcast, so wasm uses concept:static-host-mode instead
scope: link local by construction, which satisfies rule:unauthenticated-admission-requires-scope-or-capability
version_filter:
  decided: a beacon of this game whose data:game-version differs stays out of the browse list
  why: >
    once the identity filter keeps other games out, what a relaxed filter
    would add is one's own game at the wrong build — an entry whose only
    useful response is to update, which the lobby cannot offer. An entry a
    player cannot act on is worse than no entry.
  hidden_is_not_discarded: >
    the listener drops the beacon with a bare continue, so a developer chasing
    an empty lobby has nothing to read. Counting it, or recording it through
    the observe.Metrics and observe.Log api:lan-preset already builds for the
    netplay server, keeps the list clean and the cause findable.
  not_the_safety_control: a join is refused again at the seat request with 409 and again at the admission handshake, so this filter decides display alone
pairs_with: decision:no-auth-on-lan
composed_by: api:lan-preset, which pairs a beacon with a seat grant so a game needs neither
```
