---
id: rule:sight-content-owned-by-game
type: rule
title: Sight Content Owned By Game
---
Framework owns the projection mechanism; the game decides what each concept:sight contains.

```yaml
framework_provides: concept:agent-view retention, incremental diffing, delivery
game_provides: visibility predicate and field selection per agent
consequence: information volume is a game tuning knob, not a framework constant
policy: policy:sight-scoped-information still applies at every setting
```
