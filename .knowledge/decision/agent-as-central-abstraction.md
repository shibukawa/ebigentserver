---
id: decision:agent-as-central-abstraction
type: decision
title: Agent Is The Central Abstraction
---
Model every participant as concept:agent rather than modeling players and AI separately.

```yaml
decided: yes
alternatives_rejected: player class plus separate bot controller hierarchy
consequences:
  - human, AI, replay, and remote participants are interchangeable
  - concept:observation becomes mandatory instead of direct state access
  - simulation, testing, and training reuse the live session path
serves: vision:agent-session-runtime, requirement:unified-agent-model
```
