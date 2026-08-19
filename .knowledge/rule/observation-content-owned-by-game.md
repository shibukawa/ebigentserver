---
id: rule:observation-content-owned-by-game
type: rule
title: Observation Content Owned By Game
---
Framework owns the projection mechanism; the game decides what each concept:observation contains.

```yaml
framework_provides: concept:agent-view retention, incremental diffing, delivery
game_provides: visibility predicate and field selection per agent
consequence: information volume is a game tuning knob, not a framework constant
policy: policy:observation-scoped-information still applies at every setting
```
