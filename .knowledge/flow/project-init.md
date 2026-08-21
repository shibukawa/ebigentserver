---
id: flow:project-init
type: flow
title: Project Init
---
`ebigent init` from an empty directory to a project that already runs.

```yaml
questions:
  - 1: game name, which also names the directory init creates when none was given, then the go module path
  - 2: concept:participant-shape — solo, duo, or multi
  - 3: the seat count, asked only under multi; solo and duo fix their own
  - 4: whether every seat reads the same screen content, asked only when there is more than one seat; see concept:view-arrangement
  - 5: concept:synchronization-mode, asked only when a link exists, offering only the modes the seat count can reach
  - 6: which agentic environment the developer works in, asked last because it is about tooling rather than the game; it decides where the analysis skill is written and is never recorded
not_asked:
  - the build targets, which follow from whether a link exists rather than being chosen
  - where the traffic goes, which is a data:run-config value a project changes without regenerating
  - the AI pipeline, always written per decision:ai-pipeline-always-scaffolded
writes:
  - ebigent.toml carrying both data:build-config and data:run-config sections
  - one cmd entry point per generated concept:build-target, with the server directory holding both linkage forms behind the listen tag
  - the game rules package, its data:session-tuning-profile declaration, and its tests
  - an import graph test holding the project to rule:engine-import-confined-to-client-entry
  - the chip library, the corpus directory, and the analysis skill at the path the chosen environment reads
then:
  - run go mod tidy and tinybind codegen
  - build once and report the result
non_interactive: every answer is also a CLI option, so init runs unattended in a test
refuses: a directory already holding any file it would write, rather than overwriting work
serves: requirement:project-scaffolding
```
