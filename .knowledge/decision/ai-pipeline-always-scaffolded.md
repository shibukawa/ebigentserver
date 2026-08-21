---
id: decision:ai-pipeline-always-scaffolded
type: decision
title: AI Pipeline Always Scaffolded
---
`ebigent init` never asks whether the AI pipeline is wanted; it is always generated.

```yaml
decided: yes
always_written: an episode recording destination in data:run-config, a chip library file, a corpus directory, and the analysis skill
skill_location_is_not_configuration: >
  the skill runs in the developer's own agentic environment
  (rule:analysis-tooling-outside-game-process), and each environment reads
  a fixed path of its own — .claude/skills for one, .agents/skills
  otherwise. init asks which environment and writes there, so the
  environment finds the skill by its own convention and nothing has to be
  told where it went. A path setting would only be a second place for the
  answer to be wrong.
why: recording must already be in place before there is any history to distill; a project that opts in later has no corpus to learn from
cost_when_unused: an empty corpus directory and one unused config key
serves: requirement:behavior-learning-from-logs
```
