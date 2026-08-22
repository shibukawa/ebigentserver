---
id: requirement:config-codegen
type: requirement
title: Config Code Generation
---
Protocol-level settings become Go constants at build, so a built artifact reads no file for anything fixed before it existed.

```yaml
new_verb: generate — api:game-cli has init, build, config, analyze, merge, doctor, version and five pending verbs, and no generate
verbs:
  generate: reads ebigent.toml, emits the protocol constants and the codecs of requirement:cborbind-migration, writes nothing else
  build: runs generate, then compiles the named concept:build-target
  dev: runs generate on every change flow:dev-rebuild-loop detects, then rebuilds
  why_build_runs_it: a target compiled against stale constants is the failure this exists to prevent, so generation is not a step a person remembers
emits:
  - the protocol constants of requirement:config-file-shape, including the identity pair of decision:module-path-is-game-identity
  - the seat composition api:roster validates against
  - the per-stage codecs, see requirement:cborbind-migration
does_not_emit: anything at the run level; those stay bound by configbind per rule:config-tier-placement
idempotent: same input, same bytes, so a generated tree is diffable and a no-op run changes nothing
verified_by: editing a protocol key and rebuilding changes the artifact; editing it without rebuilding changes nothing, and no run-time lookup exists to make it appear to
```
