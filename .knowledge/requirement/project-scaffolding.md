---
id: requirement:project-scaffolding
type: requirement
title: Project Scaffolding
---
`ebigent init` writes a project that compiles and runs before any hand editing.

```yaml
wizard_axes:
  - concept:participant-shape, which fixes or asks the seat count
  - whether a machine may seat several players, the code visible half of concept:view-arrangement
  - concept:synchronization-mode, only when a link exists
not_asked:
  - the build targets, which follow from whether a link exists
  - the host and the transport, which are data:run-config values, see concept:deployment-combination
  - the AI pipeline, always scaffolded per decision:ai-pipeline-always-scaffolded
outputs:
  - one cmd entry point per generated concept:build-target, see decision:entry-points-over-build-tags; the server directory holds both linkage forms behind the listen tag
  - one ebigent.toml carrying both the data:build-config and data:run-config sections
  - a game rules package parameterised by seat count, holding a data:session-tuning-profile declaration
  - an import graph test holding the project to rule:engine-import-confined-to-client-entry
  - chip library, corpus directory, and analysis skill folder
  - go.mod with a pinned toolchain, a .gitignore, and codegen already run
default_game: a playable one-button flyer on system:ebitengine, every seat through one pipe field
default_game_rationale: it puts the constraints on screen as working code rather than prose — a realtime tick loop, fixed point physics under rule:no-float-in-simulation, a seeded stream inside the world under rule:shared-rng-seed, and an engine reachable only from the client entry
one_placeholder_for_every_style: a slower game is not a reason for a different sample, since data:presence-message travels in realtime whatever the turns do, so the same flyer teaches the loop every project ends up running
acceptance: the generated project builds, its own tests pass, and the headless target plays a full match with no further edits
flow: flow:project-init
model: decision:mirror-popcornweb-dx
```
