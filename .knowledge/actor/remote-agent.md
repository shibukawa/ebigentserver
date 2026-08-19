---
id: actor:remote-agent
type: actor
title: Remote Agent
---
concept:agent whose decision loop runs in another process and reaches the session over a transport.

```yaml
transport: api:transport-interface
admission: flow:session-admission with data:session-ticket
opacity: session cannot tell whether the far side is human or bot
trust: treated as untrusted, constrained by policy:observation-scoped-information
```
