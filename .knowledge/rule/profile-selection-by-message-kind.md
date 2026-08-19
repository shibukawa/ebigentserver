---
id: rule:profile-selection-by-message-kind
type: rule
title: Encoding Profile Selection By Message Kind
---
Each message kind maps to a fixed encoding profile.

```yaml
mapping:
  - kind: data:player-input
    profile: concept:cbor-wire-profile
  - kind: data:game-event
    profile: concept:cbor-wire-profile
  - kind: data:state-delta
    profile: wire or world profile by size and schema stability
  - kind: data:snapshot
    profile: concept:cbor-world-profile
  - kind: data:episode-log
    profile: concept:cbor-world-profile, json accepted
  - kind: debug output
    profile: json
  - kind: control plane api
    profile: json
```
