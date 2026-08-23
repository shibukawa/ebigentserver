---
id: policy:sight-scoped-information
type: policy
title: Sight Scoped Information
---
An agent receives only its own concept:sight, never concept:world-state.

```yaml
covers: term:fog-of-war, sight range, information based AI difficulty, cheat prevention
applies_to: all agents including actor:remote-agent and local bots
consequence: a compromised client cannot read hidden state because it was never sent
presence_caveat: data:presence-message feels cosmetic but leaks intent; a cursor or look direction reveals where an opponent is attending, so it passes the same visibility filter
```
