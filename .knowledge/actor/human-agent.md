---
id: actor:human-agent
type: actor
title: Human Agent
---
concept:agent driven by a person through rendering and input devices.

```yaml
input_path: concept:observation to system:ebitengine render to human to keyboard, mouse, gamepad to api:input-adapter to concept:action
latency: frame bound, must fit realtime tick
present_in: concept:standalone-mode, concept:listen-server-mode, client of concept:dedicated-server-mode
```
