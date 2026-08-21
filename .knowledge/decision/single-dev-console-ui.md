---
id: decision:single-dev-console-ui
type: decision
title: Single Dev Console UI
---
One local web UI holds both runtime inspection and behavior authoring.

```yaml
decided: yes
tabs: the ui:dev-console panes plus the ui:behavior-tree-editor and ui:chip-benchmark surfaces
why: a chip is judged on recorded evidence, and that evidence is the same situation the runtime panes render; two UIs meant copying an episode id and tick between them by hand
launch:
  - the dev verb of api:game-cli serves the full UI against a live session
  - the edit verb serves the same UI with only the authoring tabs when no session is running
unchanged: rule:generated-behavior-requires-approval still gates every write to the chip library
```
