---
id: system:tinybind
type: system
title: Tinybind
---
Existing Go code generation and binding library shared by the framework and the control plane.

```yaml
existing_features: struct analysis, json binding, config binding, cli binding
withdrawn_upstream:
  as_of: tinybind-go v0.5.21 and tinygodriver v1.2.9; this repository still pins v0.5.17 and v1.2.6, so nothing is broken yet
  gone_from_tinybind_go: >
    the whole cborbind package and its generator — GenerateWireCodec,
    GenerateWorldCodec, GenerateWireDelta, GenerateWorldDelta, and the
    generator files behind them. What remains under the cbor name is http
    body negotiation, which is unrelated. Six msg packages and fifteen
    declarations in this repository depend on the removed API.
  gone_from_tinygodriver: cbor.Wire() and cbor.World(), replaced by Canonical() and Deterministic(), which are generic CBOR restrictions rather than game profiles
  and_a_split_worth_keeping: >
    cbor.Profile used to carry format restrictions and resource limits
    together and now carries only the restrictions; the limits moved to
    DecoderOptions, chosen per deployment. Upstream's reason is the axis this
    framework calls concept:configuration-scope — mixing them makes a
    deployment decision look like a protocol change. The limits it dropped are
    the ones data:runtime-resource-budget already holds.
  rebuildable: the removed profiles are expressible as Profile literals; the doc comment on the new type spells the wire one out field by field
framework_must_own:
  - the concept:cbor-wire-profile and concept:cbor-world-profile literals
  - delta and patch generation for decision:framework-side-delta-generation
  - scale aware fixed point encode and decode, see decision:fixed-point-numeric-representation
  - determinism validation, see rule:codegen-rejects-nondeterministic-types
  - data:game-version derived from the generated schema
position: shared base under both this framework and system:popcornweb
decision: decision:reuse-tinybind-codegen, whose cbor half no longer has an upstream to reuse
```
