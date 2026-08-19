---
id: concept:signaling-method
type: concept
title: Signaling Method
---
How two endpoints learn each other's connection information before a transport exists.

```yaml
separate_from: concept:execution-topology and api:transport-interface, which assume a connection is already possible
methods:
  - name: none
    use: concept:standalone-mode, in process transport
  - name: control_plane
    use: concept:dedicated-server-mode, endpoint arrives inside data:session-ticket
  - name: lan_discovery
    use: native builds on one network, see api:lan-discovery
    unavailable_in: browser builds
  - name: manual_token
    use: concept:static-host-mode, offer and answer carried out of band by the players
    unavailable_in: nothing, works wherever system:webrtc works
selection: build target and deployment, not game genre
```
