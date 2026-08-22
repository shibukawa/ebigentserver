---
id: requirement:config-codegen
type: requirement
title: Config Code Generation
---
Protocol-level settings become Go constants at build, so a built artifact reads no file for anything fixed before it existed.

```yaml
status: done — generate exists, build runs it, and it emits both the protocol constants and the per-package deltas
verbs:
  generate: reads ebigent.toml, emits the protocol constants and the codecs of requirement:cborbind-migration, writes nothing else
  build: runs generate, then compiles the named concept:build-target
  dev: runs generate on every change flow:dev-rebuild-loop detects, then rebuilds
  why_build_runs_it: a target compiled against stale constants is the failure this exists to prevent, so generation is not a step a person remembers
emits:
  - the protocol constants of requirement:config-file-shape, including the package half of decision:module-path-is-game-identity
  - the seat composition api:roster fills, with the team division already resolved per seat
  - the delta and schema fingerprint of every package declaring a world state, see requirement:cborbind-migration
declaration_is_the_call: >
  which packages hold world states is not configured. A type asked for the
  map shape is one; asking is the declaration, the same idiom tinybind
  adopted in v0.5.23. A game that grows a stage adds a package and nothing
  else, which is what decision:codec-set-per-stage needs.
discovery_order_is_the_wire: >
  the fingerprint covers the structs in the order they are read, so
  discovery sorts and that order is canonical. Generating from a
  hand-typed order produced a different fingerprint for the same types,
  which the regeneration check caught.
does_not_emit: anything at the run level; those stay bound by configbind per rule:config-tier-placement
idempotent: same input, same bytes, so a generated tree is diffable and a no-op run changes nothing
verified_by: editing a protocol key and rebuilding changes the artifact; editing it without rebuilding changes nothing, and no run-time lookup exists to make it appear to
```
