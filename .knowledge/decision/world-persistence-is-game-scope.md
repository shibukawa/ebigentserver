---
id: decision:world-persistence-is-game-scope
type: decision
title: World Persistence Is Game Scope
---
Saving concept:world-state across process restarts is the game's responsibility; the framework only provides snapshots.

```yaml
decided: yes
framework_provides: data:snapshot serialization via concept:cbor-world-profile, already versioned by data:protocol-version
game_decides: whether to persist, where, how often, and migration between saved versions
rationale:
  - a match game never needs it, a persistent world always does; no default fits both
  - storage choice is deployment specific, which is control plane adjacent territory
boundary: a saved world reloads through the normal session start path, seeding from the stored data:snapshot
related: the persist_entity option of concept:agent-departure-policy assumes the session outlives the player, not the process
```
