---
id: flow:llm-teacher-episode-generation
type: flow
title: LLM Teacher Episode Generation
---
Generate training episodes by letting a language model play offline at controlled speed.

```yaml
flow:
  trigger: offline training run in concept:training-mode
  steps:
    - id: serialize
      actor: concept:session
      action: render concept:sight into model-readable form
    - id: reason
      actor: actor:llm-agent
      action: choose concept:action, taking seconds if needed
    - id: hold
      actor: concept:game-time-control
      action: pause or slow game time until the decision returns
    - id: advance
      actor: concept:session
      action: apply the action and continue
    - id: log
      actor: concept:session
      output: data:episode-log
```
