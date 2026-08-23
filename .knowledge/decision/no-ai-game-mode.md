---
id: decision:no-ai-game-mode
type: decision
title: No AI Branch In Game Rules
---
Game rules never branch on whether a slot is driven by a human; an all-AI run mode is a session configuration, not a rule variant.

```yaml
decided: yes
forbidden: rules, actions, sights, or scoring that differ by controller kind
allowed_and_wanted:
  - an all ai run mode that fills every concept:player-slot with agents and plays unattended
  - its purpose is bulk data:episode-log harvesting, see concept:training-farm
  - no rendering, no realtime pacing, see concept:training-mode
distinction: the mode chooses who occupies the slots; the game still cannot ask what occupies them
game_defines: concept:player-slot set, roles, legal concept:action set, concept:sight projection, rules, data:evaluation-signal
follows_from: decision:agent-as-central-abstraction
enables_without_extra_work:
  - human vs human, human vs ai, and ai vs ai from one implementation
  - mixed human and ai teams
  - takeover of a slot mid match, see concept:agent-departure-policy
review_test: a conditional on controller kind inside game logic means the abstraction leaked; a launch flag selecting controllers does not
```
