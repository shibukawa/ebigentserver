---
id: decision:ai-pipeline-always-scaffolded
type: decision
title: AI Pipeline Always Scaffolded
---
`ebigent init` never asks whether the AI pipeline is wanted; it is always generated.

```yaml
decided: yes
always_written: an episode recording destination in data:run-config, a chip library file, a corpus directory, and the analysis skill folder
why: recording must already be in place before there is any history to distill; a project that opts in later has no corpus to learn from
cost_when_unused: an empty corpus directory and one unused config key
serves: requirement:behavior-learning-from-logs
```
