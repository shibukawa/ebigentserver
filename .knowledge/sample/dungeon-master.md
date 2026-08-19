---
id: sample:dungeon-master
type: sample
title: Dungeon Master
---
Strongly asymmetric one versus many, where one slot sees the whole map and the others see almost nothing.

```yaml
players: 3 to 6, one versus n
new_capability: role scoped world views that differ in kind, not just in radius
asymmetry:
  dungeon_master: full map, places traps and monsters
  adventurers: local visibility only
timing: slow real time in concept:sample-acceptance-matrix; turn based is an optional non coverage variant
synchronization: hybrid; commands upstream, projected world downstream
visibility: role scope, the strongest test of concept:visibility-scope
exercises:
  - the projection path world to view per slot to serialize to client, rather than serialize then broadcast
  - policy:observation-scoped-information as a security boundary between players
  - data:visibility-annotation carrying genuinely different field sets per slot
  - all four controller combinations: human or AI dungeon master against human or AI party
why_it_matters: a naive broadcast architecture cannot express this game at all, so it validates the projection design
```
