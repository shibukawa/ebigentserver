---
id: decision:dual-mode-agent-pacing
type: decision
title: Dual Mode Agent Pacing
---
api:agent-interface supports both agent-driven stepping and realtime non-blocking decision.

```yaml
decided: yes
modes:
  - name: agent_driven_step
    description: agent advances the clock; session waits for its concept:action
    default_for: actor:llm-agent in concept:simulation-mode
    control: concept:game-time-control step mode
  - name: realtime_nonblocking
    description: session advances on schedule; a late agent contributes no action this tick
    default_for: actor:human-agent, actor:behavior-tree-agent, actor:remote-agent
    llm_case: acceptable for turn based and strategy games where an llm is a first class player
rationale: llm play is mainly simulation, but strategy games can host an llm at runtime
constraint: mode is a session configuration, not a per agent kind hardcoding
```
