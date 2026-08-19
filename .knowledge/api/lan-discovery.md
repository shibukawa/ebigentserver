---
id: api:lan-discovery
type: api
title: LAN Discovery
---
Broadcast beacon that lets native clients find sessions on the same network without any server.

```yaml
mechanism: periodic udp broadcast or multicast beacon from the host, passive listen on the client
beacon_payload: session name, endpoint, data:protocol-version, player count, whether a data:session-ticket is required
client_action: list responding sessions, then connect through api:transport-interface
availability: native builds only; browsers cannot send udp broadcast, so wasm uses concept:static-host-mode instead
scope: link local by construction, which satisfies rule:unauthenticated-admission-requires-scope-or-capability
version_filter: hide beacons whose protocol version differs, see rule:protocol-version-must-match
pairs_with: decision:no-auth-on-lan
```
