---
id: concept:player-slot
type: concept
title: Player Slot
---
Game-defined participation position that a controller attaches to; the unit a game reasons about instead of a person.

```yaml
defined_by: the game, as part of its rules; count, roles, and abilities per slot
holds: role, team, abilities, and the entity the slot controls in concept:world-state
attachment: exactly one concept:agent at a time, of any kind
same_as: the seat claim of data:session-ticket and the enforcement point of permission:agent-seat-control
lifecycle: a slot outlives its current controller, which is what makes concept:agent-proxy-designation and ai takeover possible
consequence: game rules reference slots, never whether a human or a bot is behind one
```
