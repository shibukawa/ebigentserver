---
id: requirement:project-scaffolding
type: requirement
title: Project Scaffolding
---
`ebigent init` writes a project that compiles and runs before any hand editing.

```yaml
wizard_axes:
  - concept:execution-topology
  - concept:synchronization-mode
  - the concept:build-target set
not_asked: the AI pipeline, always scaffolded per decision:ai-pipeline-always-scaffolded
outputs:
  - one cmd entry point per selected concept:build-target, see decision:entry-points-over-build-tags
  - data:build-config plus a commented data:run-config scaffold
  - a game rules package holding a data:session-tuning-profile preset for the chosen synchronization mode
  - chip library, corpus directory, and analysis skill folder
  - go.mod with a pinned toolchain, a .gitignore, and codegen already run
acceptance: the generated project builds and `ebigent dev` runs it with no further edits
flow: flow:project-init
model: decision:mirror-popcornweb-dx
```
