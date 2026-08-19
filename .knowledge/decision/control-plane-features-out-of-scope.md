---
id: decision:control-plane-features-out-of-scope
type: decision
title: Control Plane Features Out Of Scope
---
Framework does not implement lobby, matchmaking, ranking, inventory, or other concept:control-plane features.

```yaml
decided: yes
in_scope: concept:agent, concept:session, simulation, transport, synchronization, concept:world-state, replay, AI
out_of_scope: authentication, lobby, party, matchmaking, ranking, profile, inventory, economy, achievement, quest, tournament, match history, social, liveops
boundary: flow:session-admission
```
