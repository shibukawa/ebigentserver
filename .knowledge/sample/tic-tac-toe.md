---
id: sample:tic-tac-toe
type: sample
title: Tic-Tac-Toe
---
Smallest possible session: two slots, strict turns, request and response, terminal conditions.

```yaml
players: 2 competitive
new_capability: a concept:session exists at all, with rooms, slots, turn order, and an end condition
timing: turn based, no tick loop; concept:game-time-control runs in step mode
synchronization: command oriented; the board is small enough to send whole, so no data:state-delta yet
visibility: global scope of concept:visibility-scope
exercises:
  - concept:player-slot and seat assignment
  - flow:session-admission end to end
  - api:agent-interface in blocking step pacing
  - data:protocol-version handshake
ai: trivial bot, present only to prove decision:no-ai-game-mode from the first sample
deliberately_absent: ticks, deltas, partial visibility, unreliable transport
```
