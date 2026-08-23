---
id: concept:agent
type: concept
title: Agent
---
Participant that receives concept:sight and produces concept:action for a concept:session.

```yaml
identity: session-visible participant slot, independent of who or what drives it
contract: api:agent-interface
variation_point: only the observation-to-action policy differs between agent kinds
kinds:
  - actor:human-agent
  - actor:script-bot-agent
  - actor:behavior-tree-agent
  - actor:llm-agent
  - actor:rl-agent
  - actor:remote-agent
  - actor:replay-agent
loop: flow:agent-decision-loop
```
