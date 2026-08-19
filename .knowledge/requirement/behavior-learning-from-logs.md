---
id: requirement:behavior-learning-from-logs
type: requirement
title: Behavior Learning From Play Logs
---
Recorded play must be convertible into runtime AI behavior without hand-authoring behavior trees first.

```yaml
input: data:episode-log from human or actor:llm-agent play
knowledge_axis: flow:behavior-tree-synthesis produces data:behavior-candidate for developer review
execution_axis: flow:behavior-profile-derivation produces concept:behavior-profile
review_surface: ui:behavior-tree-editor
non_goal: fully automatic behavior generation, see rule:generated-behavior-requires-approval
output: data:behavior-tree driving actor:behavior-tree-agent
```
