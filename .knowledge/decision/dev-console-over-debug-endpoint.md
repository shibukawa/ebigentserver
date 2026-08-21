---
id: decision:dev-console-over-debug-endpoint
type: decision
title: Dev Console Over Debug Endpoint
---
The console is a separate process attaching to a debug endpoint on the game process.

```yaml
decided: yes
rejected_alternative: hosting the console inside the dev parent process and reading child stdout
rejection_reason: stdout only reaches a child of the dev command, so a concept:dedicated-server-mode process, or a client on another machine, could never be inspected
consequence: one console attaches to any dev-built process regardless of concept:execution-topology or host
contract: api:dev-debug-endpoint
safety: rule:debug-endpoint-excluded-from-release
```
