---
id: concept:behavior-distillation
type: concept
title: Behavior Distillation
---
Convert slow expensive agent play into a fast cheap runtime policy.

```yaml
teacher: actor:llm-agent, slow and costly, development time only
corpus: many data:episode-log entries
student: actor:behavior-tree-agent executing data:behavior-tree, a rule policy, or a small model
path: flow:behavior-tree-synthesis for the logic, flow:behavior-profile-derivation for the tuning
human_gate: rule:generated-behavior-requires-approval, so distillation is assisted rather than automatic
runtime_cost: the student must fit the session tick budget
```
