---
id: concept:sample-progression
type: concept
title: Sample Progression
---
Ordered ladder of samples, each adding one framework capability on top of the proven ones.

```yaml
steps:
  - sample:tic-tac-toe: sessions, slots, turns, end conditions
  - sample:reversi: shared controller interface, legal actions, search AI, ai versus ai
  - sample:pong: realtime loop, ticks, authoritative simulation, snapshots
  - sample:tron: many participants, spectators, departures, deltas at scale
  - sample:cooperative-maze: teams, roles, partial information, AI teammates
  - sample:dungeon-master: strong asymmetry, role scoped world views
  - sample:rts-lite: command plus world hybrid, fog of war, large state
reading: the ladder is a dependency order, not a difficulty ranking; each step needs the one before it
backlog:
  - connect four and pong variants: no new capability, useful only as authoring examples
  - bomberman like: server authoritative events at scale, a possible substitute for sample:tron
  - cooperative defense: npc waves driven by the same api:agent-interface
  - tag or chase: asymmetric abilities without asymmetric visibility
  - werewolf like: hidden loyalty and phase structure, the one axis of concept:coverage-matrix left thin
known_gap: mixed loyalty is not covered and remains a blocked claim in concept:sample-acceptance-matrix
constraint: rule:sample-adds-one-capability governs promotion from backlog
```
