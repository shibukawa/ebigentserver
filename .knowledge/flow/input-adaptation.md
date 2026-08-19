---
id: flow:input-adaptation
type: flow
title: Engine Input Adaptation
---
Convert engine-specific device input into engine-independent game actions.

```yaml
flow:
  trigger: system:ebitengine frame update
  steps:
    - id: read
      actor: system:ebitengine
      action: sample keyboard, mouse, gamepad state for the frame
    - id: adapt
      actor: api:input-adapter
      action: map device state to concept:action for the current term:tick
    - id: submit
      actor: actor:human-agent
      output: concept:action, encoded as data:player-input when remote
rule: rule:no-engine-input-in-game-logic
consequence: bots and replays produce the same action type without an engine
```
