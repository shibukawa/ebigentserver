---
id: concept:visibility-scope
type: concept
title: Visibility Scope
---
Named projection rule deciding which part of concept:world-state reaches a given concept:player-slot.

```yaml
generalizes: term:fog-of-war, which is one scope among several
scopes:
  - self: only what this slot perceives
  - team: shared among allied slots, hidden from opponents
  - role: tied to an asymmetric role, for example a dungeon master seeing the whole map
  - spectator: usually wider than any player, sometimes delayed to prevent relaying
  - global: everything, used by turn based games with open state
implementation: the visibility predicate of rule:observation-content-owned-by-game, materialized by concept:agent-view
annotation: each projection declares itself in data:visibility-annotation
security_note: scopes are a boundary between players, not a display filter; hidden data is never sent, see policy:observation-scoped-information
```
