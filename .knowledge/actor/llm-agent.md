---
id: actor:llm-agent
type: actor
title: LLM Agent
---
concept:agent that reasons over a serialized concept:sight to choose concept:action.

```yaml
latency: seconds per decision
primary_use: offline teacher in flow:llm-teacher-episode-generation, stepping the clock itself
runtime_use: first class player in turn based and strategy games under realtime pacing
pacing: decision:dual-mode-agent-pacing
observation_form: text serialization of concept:sight
output: data:episode-log for concept:behavior-distillation
```
