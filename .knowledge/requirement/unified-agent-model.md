---
id: requirement:unified-agent-model
type: requirement
title: Unified Agent Model
---
Session must treat human, bot, remote, and replay participants through one concept:agent interface.

```yaml
interface: api:agent-interface
covers:
  - actor:human-agent
  - actor:script-bot-agent
  - actor:behavior-tree-agent
  - actor:llm-agent
  - actor:rl-agent
  - actor:remote-agent
  - actor:replay-agent
consequence: swapping a human for a bot requires no concept:session change
authoring_consequence: decision:no-ai-game-mode, so a game never branches on controller kind
attachment_point: concept:player-slot
```
