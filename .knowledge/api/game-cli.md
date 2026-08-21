---
id: api:game-cli
type: api
title: Ebigent CLI
---
Framework toolchain command set, shipped as the single `ebigent` binary and separate from the control plane CLI.

```yaml
binary: ebigent
project_root: located by walking upward to data:build-config
verbs:
  - name: init
    action: run flow:project-init, scaffolding a project from a wizard
  - name: dev
    action: run flow:dev-rebuild-loop and serve ui:dev-console
  - name: build
    action: produce a concept:build-target artifact by building its entry point, see decision:entry-points-over-build-tags
  - name: run
    action: start a built target with a data:run-config
  - name: config
    action: render declared configuration as a commented TOML or dotenv scaffold, or print effective values with the layer that set each
  - name: check
    action: report stale or missing generated files
  - name: doctor
    action: report toolchain and environment problems
  - name: simulate
    action: run concept:simulation-farm workloads, writing data:episode-log
  - name: replay
    action: play back an episode through actor:replay-agent
  - name: export
    action: materialize a recorded corpus into analysis ready JSONL or parquet
  - name: analyze
    action: run metric:balance-signals queries over a corpus, see rule:analysis-tooling-outside-game-process
  - name: merge
    action: fold validated analyzer proposals into a chip library as a reviewable diff
  - name: train
    action: run flow:behavior-tree-synthesis and flow:behavior-profile-derivation
  - name: loop
    action: run concept:continuous-match-loop until a termination condition is met
  - name: generate
    action: emit Go source for a data:behavior-tree and its data:derived-predicate set
  - name: edit
    action: open ui:dev-console with only its authoring tabs, see decision:single-dev-console-ui
  - name: version
    action: print the toolchain version
verb_mechanism: one configbind SubCommand per verb, with options, positionals, and usage text generated from a struct
codegen_dependency: system:tinybind
unification: decision:one-ebigent-binary, serving requirement:unified-toolchain-binary
separation: decision:separate-game-cli
```
