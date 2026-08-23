---
id: data:visibility-annotation
type: data
title: Visibility Annotation
---
Game-emitted declaration of what an agent could perceive at a decision point.

```yaml
emitted_by: the game, explicitly, not inferred by the framework
rationale: only the game knows why something was visible, see rule:sight-content-owned-by-game
fields:
  - sight_schema: which fields existed for this agent at this tick
  - visible_entities: ids the agent could perceive
  - staleness: entities last seen earlier, still remembered, no longer confirmed
  - derived: values the agent computed rather than received
  - affordances: options the interface actually offered, which bounds what the player could have chosen
makes_checkable: rule:analysis-restricted-to-visible-fields, and the sight only constraint on data:behavior-candidate
without_it: an analyzer cannot tell a hidden field from an unused one, and proposes conditions the runtime agent can never evaluate
```
