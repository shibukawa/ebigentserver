---
id: ui:dev-console
type: ui
title: Dev Console
---
Local web UI for a running development session, and the single surface for behavior authoring.

```yaml
primary_panes:
  - timeline: tick duration against budget, missed ticks, send rate, snapshot and delta bytes, each shown against the data:session-tuning-profile value that set it
  - state: the concept:world-state tree at the selected tick, with each concept:agent-view rendered as what that agent could see, which is how rule:analysis-restricted-to-visible-fields is checked by eye
  - decisions: the data:decision-record stream per concept:agent — observation in, concept:action out, with the reason
secondary: session and network signals reuse api:runtime-observability rather than getting a separate design
authoring_tabs: ui:behavior-tree-editor and ui:chip-benchmark, see decision:single-dev-console-ui
time_control: pause, step, and time_scale issued through api:dev-debug-endpoint
selection_model: one tick cursor shared by every pane, so selecting a decision selects the state that produced it
served_by: api:game-cli
```
