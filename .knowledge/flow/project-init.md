---
id: flow:project-init
type: flow
title: Project Init
---
`ebigent init` from an empty directory to a project that already runs.

```yaml
steps:
  - 1: ask concept:execution-topology
  - 2: ask concept:synchronization-mode, offering only modes the chosen topology supports
  - 3: ask the concept:build-target set, warning when a target drops a capability, such as wasm excluding api:lan-discovery
  - 4: write ebigent.toml as data:build-config and a commented data:run-config scaffold
  - 5: write one cmd entry point per target, the game rules package, and a data:session-tuning-profile preset matching the synchronization mode
  - 6: write the AI pipeline scaffold, see decision:ai-pipeline-always-scaffolded
  - 7: run go mod tidy and tinybind codegen
  - 8: build once and report the result
non_interactive: every answer is also a CLI option, so init runs unattended in a test
serves: requirement:project-scaffolding
```
