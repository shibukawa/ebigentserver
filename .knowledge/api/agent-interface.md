---
id: api:agent-interface
type: api
title: Agent Interface
---
Contract every concept:agent implements so concept:session can host any agent kind.

```yaml
operations:
  - name: observe
    input: concept:observation for the current term:tick
  - name: decide
    output: concept:action or none
  - name: lifecycle
    covers: join, leave, session end, session abort
    ordering: concept:session-lifecycle
pacing: two modes supported, see decision:dual-mode-agent-pacing
  - agent driven step: session blocks until decide returns
  - realtime non blocking: no action means no input this tick
constraints:
  - no access to concept:world-state
  - decision timing governed by concept:game-time-control
  - decide accepts cancellation when its session leaves running state
implemented_by: actor:human-agent, actor:script-bot-agent, actor:behavior-tree-agent, actor:llm-agent, actor:rl-agent, actor:remote-agent, actor:replay-agent
```
