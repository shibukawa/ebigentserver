---
id: data:evaluation-signal
type: data
title: Evaluation Signal
---
Session-computed judgement of how a concept:player-slot is doing, delivered with concept:observation and recorded for analysis.

```yaml
purpose: without it an agent has no criterion; it can act legally and still not know whether it is winning
fields:
  - score: the game visible score, points, resources, health
  - progress: distance to the win condition, normalized where the game can define it
  - evaluation: heuristic value of the current position, the game equivalent of a chess eval
  - reward_delta: change since the previous decision point, the per decision credit signal
  - terminal: set only at the end, win, lose, draw, or abandoned
defined_by: the game, like every other content decision, see rule:observation-content-owned-by-game
computed_by: rule:evaluation-computed-by-session
scoped_by: rule:evaluation-respects-visibility-scope
versioning: an evaluation_version travels in the data:episode-log header, since changing the function invalidates comparisons across a corpus
delivered_to: every controller equally, so a human client may render or ignore it, preserving decision:no-ai-game-mode
```
