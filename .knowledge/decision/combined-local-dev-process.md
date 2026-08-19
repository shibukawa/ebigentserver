---
id: decision:combined-local-dev-process
type: decision
title: Combined Local Development Process
---
Local development may run client, session, and realtime transport in one process even though production splits them.

```yaml
decided: yes
example: concept:listen-server-mode process containing system:ebitengine, actor:human-agent, concept:session, simulation, transport
excluded: system:popcorn-wave is not embedded into the framework for this convenience
entry_point: api:game-cli dev
```
