---
id: requirement:cborbind-migration
type: requirement
title: Cborbind Migration
---
Move to tinybind-go v0.5.23, whose cborbind is a different API from the pinned v0.5.17 one, and take over what it no longer generates.

```yaml
upstream_state:
  gone_then_back: cborbind is absent in v0.5.21 and present again in v0.5.23 as one file with a new API
  no_declaration: >
    the v0.5.17 form declared intent — GenerateWireCodec[T], GenerateWorldDelta[T].
    v0.5.23 has none: calling AppendCBORInArrayTo or DecodeCBORInArrayFrom is
    itself the ask, and the generator reads the call site.
  no_profile: >
    the wire and world profiles are gone from both tinybind-go and
    tinygodriver. What replaces them is container shape, chosen by method name.
shape_replaces_profile:
  array: positional, member names off the wire, adding a field changes the length so both ends rebuild together — this is concept:cbor-wire-profile
  map: keyed, an unknown key is skipped so the two ends may ship separately — this is concept:cbor-world-profile
  consequence: rule:profile-selection-by-message-kind maps a message kind to a shape rather than to a profile object
framework_must_now_own:
  delta_generation: >
    v0.5.23 emits no diff, apply, or patch. decision:framework-side-delta-generation
    stops being a choice and becomes work: statesync.Codec's Diff, AppendDelta,
    DecodeDelta, and ApplyDelta have no upstream source.
  clone: the one statesync.Codec field always hand-written, and the one whose hand-written form aliases a slice; generating it removes that defect class
  float_refusal: >
    no profile means no float rejection at the encoder. rule:no-float-in-simulation
    and decision:fixed-point-numeric-representation now need a generation-time
    check of their own, since v0.5.23 accepts float64 as an ordinary field type.
  size_limits: the bounds the old profile carried belong to data:runtime-resource-budget, which already holds their kind
migration_in_this_repository:
  tag_change: member names come from the json tag; six msg packages carry cbor tags with explicit keys today
  call_sites: fifteen GenerateWireCodec and GenerateWorldDelta declarations across examples, samples, and the tutorial
  driver: cborbind itself imports nothing; only generated code names the driver, so a package calling no entry point links no CBOR
  tinygo: the generated codecs build for wasm and wasip1, which concept:build-target still requires
verified_by: every msg package regenerating under v0.5.23 with byte-identical wire output for unchanged types, and a float field in a simulation type failing generation
```
