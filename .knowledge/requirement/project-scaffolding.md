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
  - an import graph test holding the project to rule:engine-import-confined-to-client-entry
  - chip library, corpus directory, and analysis skill folder
  - go.mod with a pinned toolchain, a .gitignore, and codegen already run
default_game: a playable one-button flyer on system:ebitengine, two agents through one pipe field
default_game_rationale: it exercises what a turn-based placeholder cannot — a realtime tick loop, fixed point physics under rule:no-float-in-simulation, a seeded stream inside the world under rule:shared-rng-seed, and an engine confined to the client entry — so the constraints are visible as working code rather than as prose
acceptance: the generated project builds, its own tests pass, and the headless target plays a full match with no further edits
flow: flow:project-init
model: decision:mirror-popcornweb-dx
```
