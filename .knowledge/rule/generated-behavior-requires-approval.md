---
id: rule:generated-behavior-requires-approval
type: rule
title: Generated Behavior Requires Approval
---
No data:behavior-candidate reaches runtime without a developer accepting it.

```yaml
rationale:
  - an analyzer optimizes for reproducing logged play, which includes mistakes and exploits
  - a statistically strong rule can still be unfun, degenerate, or an unintended strategy
  - the developer owns what the game feels like, and that is not derivable from the logs
gate: acceptance in ui:behavior-tree-editor writes the node into data:behavior-tree
rejected_candidates: kept with the reason, so the next analysis run does not re-propose them
autonomy_limit: the analyzer may propose, rank, and explain, but never commit
```
