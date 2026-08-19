---
id: flow:behavior-profile-derivation
type: flow
title: Behavior Profile Derivation
---
Measure how a recorded player executes, producing the tuning vector rather than the decision logic.

```yaml
flow:
  trigger: sufficient episode corpus exists
  steps:
    - id: collect
      action: gather data:episode-log from human and actor:llm-agent play
    - id: measure
      action: extract timing, risk taking, and prediction quality per decision point
    - id: profile
      output: concept:behavior-profile vector
    - id: apply
      action: instantiate actor:behavior-tree-agent with the profile
produces: the execution axis only
companion: flow:behavior-tree-synthesis produces the knowledge axis, the tree itself
serves: requirement:behavior-learning-from-logs
```
