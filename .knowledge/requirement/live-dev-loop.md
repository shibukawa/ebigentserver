---
id: requirement:live-dev-loop
type: requirement
title: Live Development Loop
---
One command generates, builds, runs, and restarts the game on every source change.

```yaml
command: the dev verb of api:game-cli
cycle: flow:dev-rebuild-loop
covers: codegen, build, run, restart, and ui:dev-console under a single Ctrl-C
process_shape:
  - single process for a topology that allows it, see decision:combined-local-dev-process
  - separate child processes when the selected concept:execution-topology splits client and server
failure_behavior: a build failure keeps the running process alive and reports the error in the console
model: decision:mirror-popcornweb-dx
```
