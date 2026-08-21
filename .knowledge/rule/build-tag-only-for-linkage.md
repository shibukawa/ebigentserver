---
id: rule:build-tag-only-for-linkage
type: rule
title: Build Tag Only For Linkage
---
A build tag is justified only when code cannot or must not be linked; everything else is runtime configuration.

```yaml
justified:
  - platform capability, for example api:lan-discovery using udp, absent under js and wasm
  - toolchain constraints, for example anything needing cgo, see rule:analysis-tooling-outside-game-process
  - optional heavy dependencies a game may exclude
automatic_in_go: GOOS and GOARCH constraints cover the wasm split with no custom tag
not_justified:
  - concept:game-time-control speed, which is a runtime value, not a compile time one
  - concept:execution-topology as a runtime value, which is what data:run-config topology is
  - concept:synchronization-mode, concept:delta-baseline-policy, concept:ack-transmission-policy
  - log destination, agent roster, tick rate
default: runtime configuration through data:run-config
renderer_linkage_is_justified:
  claim: whether an artifact links system:ebitengine is a build fact, not a run value, so the listen and headless forms of one server are a tag rather than two entry points
  why_it_passes_the_test: they cannot share a process — a headless server must not link the engine at all under rule:engine-import-confined-to-client-entry — and the engine is exactly the "optional heavy dependency" case above
  why_it_is_not_the_rejected_alternative: decision:entry-points-over-build-tags rejects a tag threaded through library packages; this one never leaves the entry point directory, so a missing tag still fails there rather than somewhere distant
  bound: two files in one cmd directory, each supplying the same small set of functions; a third variant means the split was wrong
tag_axes:
  rule: one entry point carries at most one tag axis that changes its shape
  why: decision:entry-points-over-build-tags rejects tags partly because combinations multiply and only some are ever compiled; a second shape-changing axis is exactly that failure
  the_one_axis: renderer linkage, being listen against headless
  transports_are_not_an_axis: which transport carries a session is chosen at runtime from concept:transport-capability under rule:transport-selected-by-capability, so a tag naming a transport would be a mode selector, which is what the rule forbids
  transports_need_no_exclusion_tag: the native implementations are already constrained to !js && !wasip1, so a browser build never links pion or quic-go — the automatic_in_go case above, not a custom tag
  measured: a js/wasm binary is the same size with and without either transport package imported, because neither compiles for that target
  native_size: a native server binary carrying an unused transport is a few megabytes nobody is counting
  unchanged: the topology this process plays in is still a run value, and both tag variants read the same data:run-config
test: if two variants could sensibly run in the same process, it is configuration
```
