---
id: api:game-cli
type: api
title: Game CLI
---
Framework toolchain command set, separate from the control plane CLI.

```yaml
commands:
  - name: dev
    action: run local development process, see decision:combined-local-dev-process
  - name: build
    action: produce a concept:build-target artifact by building its entry point, see decision:entry-points-over-build-tags
  - name: run
    action: start a built target with a data:run-config
  - name: simulate
    action: run concept:simulation-farm workloads, writing data:episode-log
  - name: replay
    action: play back an episode through actor:replay-agent
  - name: export
    action: materialize a recorded corpus into analysis ready JSONL or parquet
  - name: analyze
    action: run metric:balance-signals queries over a corpus, see rule:analysis-tooling-outside-game-process
  - name: train
    action: run flow:behavior-tree-synthesis and flow:behavior-profile-derivation
  - name: loop
    action: run concept:continuous-match-loop until a termination condition is met
  - name: generate
    action: emit Go source for a data:behavior-tree and its data:derived-predicate set
  - name: edit
    action: open ui:behavior-tree-editor against a data:behavior-tree
codegen_dependency: system:tinybind
decision: decision:separate-game-cli
```
