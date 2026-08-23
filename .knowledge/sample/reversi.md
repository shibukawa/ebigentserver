---
id: sample:reversi
type: sample
title: Reversi
---
Same turn structure as sample:tic-tac-toe, but the action space is large enough that controllers differ meaningfully.

```yaml
players: 2 competitive
new_capability: legal action generation shared by every controller kind, and search based AI on the same interface
timing: turn based
synchronization: command oriented
visibility: global scope
exercises:
  - legal concept:action enumeration as part of concept:sight, so a bot needs no private rule engine
  - search AI, actor:script-bot-agent, and actor:llm-agent behind one api:agent-interface
  - ai versus ai matches, first use of decision:samples-as-test-infrastructure
  - determinism of a full game from recorded actions, verifying actor:replay-agent
ai_matrix: human vs human, human vs ai, ai vs ai from one implementation
deliberately_absent: real time, partial information
```
