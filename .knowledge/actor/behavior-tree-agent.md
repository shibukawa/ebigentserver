---
id: actor:behavior-tree-agent
type: actor
title: Behavior Tree Agent
---
concept:agent driven by a behavior tree or comparable runtime policy.

```yaml
executes: data:behavior-tree, the compiled materialization of one data:agent-loadout
authoring: hand written, or accepted from analysis via flow:behavior-tree-synthesis
level: node gates of concept:skill-level-gating decide which branches exist for this instance
tuning: concept:behavior-profile decides reaction delay and execution quality
role: production runtime AI, target of concept:behavior-distillation
cost: must fit the session tick budget, unlike actor:llm-agent
```
