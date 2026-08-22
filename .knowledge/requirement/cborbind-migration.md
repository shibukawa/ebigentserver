---
id: requirement:cborbind-migration
type: requirement
title: Cborbind Migration
---
Move to tinybind-go v0.5.23, whose cborbind is a different API from the pinned v0.5.17 one, and take over what it no longer generates.

```yaml
status: done — v0.5.23 is pinned, all six msg packages regenerated, and the full suite passes
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
  determinism_gate: >
    no profile means no float rejection at the encoder, and v0.5.23's own
    analysis refuses nothing either — it lists float64, int, and uint among
    the field types it accepts. So codegen.Read is the only place
    rule:codegen-rejects-nondeterministic-types still holds: it refuses
    floats, host-width int and uint, and maps, naming the field rather than
    skipping it.
  size_limits: the bounds the old profile carried are cbor.DecoderOptions now, emitted as a variable a deployment can lower
  schema_version: v0.5.23 derives none, so codegen does — a fingerprint over type names, field names, order, and how each field is carried
migration_in_this_repository:
  tag_change: member names come from the json tag; six msg packages carry cbor tags with explicit keys today
  call_sites: fifteen GenerateWireCodec and GenerateWorldDelta declarations across examples, samples, and the tutorial
  driver: cborbind itself imports nothing; only generated code names the driver, so a package calling no entry point links no CBOR
  tinygo: the generated codecs build for wasm and wasip1, which concept:build-target still requires
wire_moved_once: >
  the array shape encodes byte for byte what concept:cbor-wire-profile did, so
  every pinned action digest survived the migration untouched. The map shape
  does not: the profile's integer labels are gone and members carry their
  names, so every world digest moved exactly once. The action digests holding
  still is what shows the encoding changed and no simulation did.
verified_by: the full suite, including the two-instance LAN match of the step2 tutorial, which exchanges generated deltas end to end
```
