---
id: concept:control-plane
type: concept
title: Control Plane
---
Non-realtime game service layer that exists beside sessions and is out of framework scope.

```yaml
functions:
  - authentication
  - lobby
  - party
  - matchmaking
  - session allocation
  - ranking
  - player profile
  - inventory
  - economy
  - achievement
  - quest
  - tournament and season
  - match history
  - social
  - liveops
implementations: system:popcorn-wave or any other
contract_with_framework: data:session-ticket inbound, data:progress-report outbound
scope: decision:control-plane-features-out-of-scope
```
