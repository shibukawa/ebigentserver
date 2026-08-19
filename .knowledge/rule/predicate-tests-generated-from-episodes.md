---
id: rule:predicate-tests-generated-from-episodes
type: rule
title: Predicate Tests Generated From Episodes
---
Every data:derived-predicate ships with tests built from recorded situations, not hand written fixtures.

```yaml
source: data:decision-record rows already contain the concept:observation the predicate must judge
generation: sample positive cases, negative cases, and boundary cases near the predicate threshold
value:
  - a regenerated or hand edited predicate that changes meaning fails immediately
  - the developer reviews concrete situations rather than abstract logic, matching ui:behavior-tree-editor evidence review
  - fixtures are real, so they include the awkward cases a hand written test would omit
maintenance: fixtures are pinned to an episode id and tick, so a failing test points at a replayable moment
```
