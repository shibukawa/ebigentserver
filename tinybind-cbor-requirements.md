# tinybind — CBOR binding requirements

Add CBOR code generation to tinybind-go, on top of the codec that shipped in
tinygodriver v1.2.5. This is `system:tinybind` `added_features` and `plan.md` Phase 0.

- Generator module: `github.com/shibukawa/tinybind-go` (v0.5.16 at time of writing)
- Runtime dependency: `github.com/shibukawa/tinygodriver/encoding/cbor` (v1.2.5)
- Proposed runtime package: `github.com/shibukawa/tinybind-go/cborbind`
- Status: **specification** — nothing generated yet

---

## 1. Purpose

### 1.1 What already exists

tinygodriver v1.2.5 shipped the codec. tinybind does not need to build any of this:

| Capability | Surface |
|---|---|
| Zero-allocation encode into a caller buffer | `AppendUint`, `AppendInt`, `AppendArrayHeader`, `AppendMapHeader`, … |
| Zero-allocation decode from a byte slice | `Reader`, reusable via `Reset`, borrowing strings |
| Width-enforcing reads | `ReadInt8`/`16`/`32`/`64`, `ReadUint8`/`16`/`32`/`64`, out of range is `ErrIntegerOverflow` |
| Named profiles | `cbor.Wire()`, `cbor.World()`, both float-refusing |
| Profile enforcement without decoding | `Profile.Validate`, `Profile.ValidateAppended` |
| Self-encoding types | `cbor.Appender{AppendCBORTo}`, `cbor.Decodable{DecodeCBORFrom}` |
| Skipping an unknown item | `Reader.Skip`, 0 allocations |
| Sub-item capture at depth | `Reader.ReadRaw`, borrowing from the input |
| Error location | `*cbor.Error` with byte offset and container route |

Measured cost, from the package README: 9.2 ns/op and 0 allocs to encode a fixed-shape
wire message; 42.4 ns/op and 0 allocs to decode it. The generated code has to stay inside
that budget, which is the main constraint on how it is emitted.

**The generator's reference output already exists too.** `encoding/cbor/codec_test.go`
carries a hand-written `playerInput` codec described in its own comment as "the shape a
generator would emit for the wire profile", together with `TestWireMessageEncodesToPinnedBytes`
pinning the exact bytes. That file is the acceptance oracle for §4, not merely an example.

### 1.2 What tinybind must add

`system:tinybind` names five additions. They are independent enough to land separately, and
they are listed here in dependency order:

| # | Capability | Section | Blocking for |
|---|---|---|---|
| 1 | CBOR codec generation for both profiles | §4 | Phase 0 completion criterion, all of Phase 3 |
| 2 | Scale-aware fixed-point encode/decode | §5 | `decision:fixed-point-numeric-representation` |
| 3 | Determinism validation at generation time | §6 | `rule:codegen-rejects-nondeterministic-types` |
| 4 | `data:protocol-version` derived from the schema | §7 | `flow:session-admission` handshake |
| 5 | Diff and patch generation | §8 | `decision:framework-side-delta-generation`, Phase 3a |

Items 1–4 are Phase 0. Item 5 is not needed until Phase 3a, but §8 exists now because the
analysis it needs is the same analysis §4 performs, and building §4 in a way that cannot
carry it would mean doing the traversal twice.

### 1.3 Out of scope

| Concern | Owner |
|---|---|
| The CBOR wire format, profiles, limits, and their enforcement | `tinygodriver/encoding/cbor` |
| Fixed-point arithmetic and scale conversion arithmetic | `github.com/shibukawa/fixmath` |
| Which profile a message kind uses | `rule:profile-selection-by-message-kind`, the game |
| Rejecting a version mismatch at the handshake | `flow:session-admission` |
| Retaining baselines, choosing one, tracking acks | `concept:delta-baseline-policy`, `api:sequence-ack-layer` |
| Visibility filtering of a delta | `concept:agent-view`, `rule:evaluation-respects-visibility-scope` |

### 1.4 Normative references

tinybind: `decision:runtime-package-boundaries`, `rule:usage-directed-generation`,
`requirement:declared-json-codec`, `requirement:json-codec-interface`,
`requirement:dynamobind-generated-item-codec`, `decision:dynamobind-static-dispatch`,
`rule:named-type-field-kind`, `rule:same-package-convention`,
`rule:generated-source-self-contained`, `decision:generated-runtime-in-module`,
`decision:reflection-free`, `requirement:tinygo-wasm`.

ebigentserver: `decision:reuse-tinybind-codegen`, `concept:cbor-wire-profile`,
`concept:cbor-world-profile`, `rule:fixed-point-on-wire`,
`rule:codegen-rejects-nondeterministic-types`, `decision:go-struct-world-state`,
`decision:framework-side-delta-generation`, `data:protocol-version`,
`rule:protocol-version-must-match`.

---

## 2. Placement — settled by existing precedent

Nothing here needs a new decision. Each point is the third instance of a pattern tinybind
already has.

- **P-1** Runtime package `github.com/shibukawa/tinybind-go/cborbind`, a transport-neutral
  leaf, per `decision:runtime-package-boundaries`. It excludes `net/http` and `database/sql`.
- **P-2** `cborbind` imports `tinygodriver/encoding/cbor`. This is the same edge
  `dynamobind -> tinygodriver/nosql/dynamodb` and `fasthttpbind -> tinygodriver/fasthttp`
  already have, and the reverse edge is already forbidden in writing:
  `tinygodriver -> tinybind-go, in any package, example, or test`.
- **P-3** Generator files mirror the DynamoDB set: `generator/cborbind.go`,
  `cborbind_emit.go`, `cborbind_generate.go`, `cborbind_decode.go`, `cborbind_types.go`.
- **P-4** **Static dispatch, no registry.** `decision:dynamobind-static-dispatch` already
  establishes that a mode may emit methods and register nothing. For CBOR this is not a
  preference: a registry lookup per message, at 60 Hz per player, is the cost §1.1 measured
  the package to avoid.
- **P-5** Generated code carries no copy of a runtime helper
  (`decision:generated-runtime-in-module`) and is self-contained otherwise
  (`rule:generated-source-self-contained`).

---

## 3. The interface is already declared, and tinybind must not redeclare it

This is the one place where the CBOR mode differs structurally from the JSON mode, and
getting it wrong is cheap to do and expensive to undo.

For JSON, tinybind declares the contract itself: `jsonbind/interface.go` holds
`Appender{AppendJSONTo}` and `Decoder{DecodeJSONFrom}`. For CBOR the contract is already
declared, in `tinygodriver/encoding/cbor`, as `Appender{AppendCBORTo}` and
`Decodable{DecodeCBORFrom}` — note `Decodable`, not `Decoder`, because `cbor.Decoder` is
the streaming reader.

- **I-1** `cborbind` declares **no** codec interface of its own. Two spellings of one
  contract means a type can satisfy the wrong one and be silently skipped, which is the
  `a_foreign_type_is_refused` failure `requirement:declared-json-codec` already had to fix
  once for JSON.
- **I-2** The generator recognizes `cbor.Appender` and `cbor.Decodable` structurally with
  `go/types`, as `requirement:json-codec-interface` does for its own pair, so a game package
  need not import `cborbind` merely to have its types admitted.
- **I-3** Emitted codecs satisfy those interfaces by delegation, the way
  `requirement:dynamobind-generated-item-codec` emits `EncodeItem`/`DecodeItem` onto the type.
- **I-4** Method-over-plan precedence, unchanged from JSON: a type carrying
  `AppendCBORTo` is encoded through it even when the run also planned that type.
- **I-5** `AppendCBORTo` returns no error, so a foreign implementation cannot be trusted to
  respect the profile. Generated code that embeds a foreign field in a wire-profile message
  must be able to check what was actually written — `cbor.Profile.ValidateAppended(dst, before)`
  exists for exactly this. Whether that check is always on, or on under a build tag that CI
  sets, is §10 open question 3.

---

## 4. Codec generation

### 4.1 Trigger

- **G-1** Declaration-driven, not call-driven. A game's message types are handed to the
  session framework, which encodes them generically; there is no `cborbind.Encode[T]` call in
  the game's own source for `rule:usage-directed-generation` to discover. This is precisely the
  case `requirement:declared-json-codec` was built for — "a type crossing a boundary with no
  generic call at the crossing".
- **G-2** The declaration names the profile, because the profile is part of the contract and
  the two ends must not disagree about it:

  ```go
  var _ = cborbind.GenerateWireCodec[PlayerInput]()
  var _ = cborbind.GenerateWorldCodec[WorldState]()
  ```

  Direction-narrowed forms (`GenerateWireEncoder`, `GenerateWireDecoder`, and the world
  equivalents) follow `requirement:declared-json-codec`'s `directions` reasoning: emitting an
  unused direction is code size on a wasm client.
- **G-3** `rule:same-package-convention` applies unchanged — the declaration lives in the
  package declaring the type, and naming a foreign type is a generation error, not silence.
- **G-4** The emitted code pins the profile it was generated for. A wire-profile codec must
  not be usable to read a world-profile message.

### 4.2 Wire profile output

- **W-1** A struct encodes as a fixed-order, fixed-length array. No field names, no map, no
  optional fields, no tags. Field order is declaration order and is part of the protocol.
- **W-2** Each field emits one sized append and one width-enforcing read: `uint32` writes
  through `AppendUint` and reads through `ReadUint32`, so a value too wide for its declared
  field is a protocol error rather than a silent truncation.
- **W-3** The decoder checks the array header count against the schema and refuses a
  mismatch. It never skips an unknown field: under the wire profile an unknown field cannot
  exist, and pretending otherwise is what `rule:protocol-version-must-match` refuses.
- **W-4** Steady state must be zero-allocation on both sides, with the destination buffer
  and the `Reader` owned by the caller and reused. `TestFixedShapeMessageIsZeroAllocationInSteadyState`
  is the shape of the gate.
- **W-5** Re-encoding what was decoded reproduces the same bytes. This is the property a
  replay compares digests on, and it is pinned in the reference file already.

### 4.3 World profile output

- **W-6** A struct encodes as a map, with deterministic bytewise key order, admitting
  optional fields and tags. Integer keys should be preferred over text keys where a stable
  field numbering exists, since `World()` permits both and integers are smaller.
- **W-7** The decoder skips an unknown field through `Reader.Skip` rather than failing, which
  is the schema tolerance this profile exists to provide.
- **W-8** Optional-field semantics must be stated, not inherited. JSON's `omitempty`/`omitzero`
  reading is an `encoding/json/v2` alignment decision with no meaning here; an omitted
  world-profile field means "unchanged or absent", which is a different question. §10 open
  question 2.

### 4.4 Foreign and composite fields

- **W-9** **The known gap must be closed.** `requirement:json-codec-interface` records under
  `not_built_yet`: "a slice or map whose element is a foreign type is still dropped, since
  `fieldTypeKind` admits only scalars and planned structs as elements". For JSON that is a
  missing nicety. For CBOR it is blocking — `concept:world-state` is entities in slices, and
  the scalar inside them is exactly the foreign fixed-point type. A silently dropped field is
  the worst available failure here, because the two ends still agree on everything they did
  encode.
- **W-10** Nested planned structs compose by appending into the same destination, at any
  depth, with no allocation. Foreign fields at depth decode through `Reader.ReadRaw`, which
  borrows from the input rather than copying it.

---

## 5. Fixed point and scale

### 5.1 The scale is per field, and the interface is per type

This is the finding that shapes the whole section, and it is worth stating before the
requirements it produces.

`rule:fixed-point-on-wire` puts the scale in the schema, per field. But `cbor.Appender` is a
method on a *type*. A position at scale 1/1024 and a velocity at scale 1/65536 are both
`fixmath.F64`, so one method cannot serve both — whichever scale it picks is wrong for the
other field. The reference `fixed64` in `codec_test.go` sidesteps this by writing a bare
`AppendInt` and saying in its comment that the scale "lives in the schema".

- **F-1** **Generated code owns the scale conversion; the interface does not.** For a field
  carrying a declared scale, the generator emits the conversion and a bare integer append at
  the field site, rather than delegating to the type's `AppendCBORTo`. The interface stays for
  types whose encoding really is self-contained.
- **F-2** The scale is declared by struct tag, following the tag-option machinery tinybind
  already has for `enum`, `check`, `dynamo`, and `firestore`:
  `` `cbor:"pos_x,scale=1024"` ``. An unknown option is a generation error, not a silently
  inert tag, matching `concept:standalone-json-codec`'s existing rule.
- **F-3** A fixed-point field with no declared scale is a generation error naming the type
  and the field. This is one of the five rejection classes of
  `rule:codegen-rejects-nondeterministic-types`, and its stated reason is that the scale is
  part of `data:protocol-version` and cannot be inferred.
- **F-4** The conversion arithmetic belongs to fixmath, whose own specification claims
  "conversion between a declared per-field wire scale and the canonical compute format" as in
  scope. tinybind emits the call; it does not implement the arithmetic, and **`cborbind` must
  not depend on fixmath** — a game may supply its own core, which `api:fixed-point-math`
  explicitly permits.
- **F-5** The conversion must be integer arithmetic. A scale change implemented as a float
  multiply would reintroduce exactly the platform variance `rule:no-float-in-simulation` bans,
  inside generated code, where nobody would look for it.
- **F-6** Which fixed-point type is in use is a project-level configuration, not a hardcoded
  import. `rule:named-type-field-kind` already handles a defined type over `int64`, which is
  what `fixmath.F64` is.

---

## 6. Determinism validation

`rule:codegen-rejects-nondeterministic-types` is a build-time gate, and it is the reason
Phase 0 says a float leak "cannot reach production as a desync".

- **D-1** Checked over the type set transitively reachable from the world-state root, per
  `decision:go-struct-world-state`. No separate annotation marks that set; reachability is
  the definition.
- **D-2** The five rejection classes, each a generation error naming the type and the field:

  | Rejected | Why |
  |---|---|
  | `float32`, `float64` | platform variance and fused multiply-add |
  | `map` | Go randomizes iteration order, so traversal and diff output vary per run |
  | `interface`, pointer to a shared value | identity and aliasing are not reproducible from a snapshot |
  | `time.Time` and wall-clock-derived values | not a function of `term:tick` |
  | fixed-point field with no declared scale | the scale is part of the protocol version |

- **D-3** A build error, never a runtime warning. It must also not be a *lint* the project can
  leave failing: `rule:generator-feature-disable` lets features be turned off, and this check
  must not be one of them for a type reached by a wire or world codec.
- **D-4** The float rejection interacts with an existing tinybind capability:
  `rule:named-type-field-kind` today happily admits `type Ratio float64`, and the named-scalar
  test suite has one. Under a CBOR profile that must fail, and the diagnostic has to name the
  underlying kind, since the declared kind looks innocent.
- **D-5** The map rejection needs an escape: `rule:codegen-rejects-nondeterministic-types`
  names "ordered slice, or generated traversal in sorted key order" as the alternative, so the
  world profile's map output (W-6) comes from generated ordered traversal, never from ranging
  a Go map.

---

## 7. Protocol version derivation

- **V-1** The generator emits one `data:protocol-version` identifier covering every generated
  schema: field order, field types and widths, declared scales, profile, and map key ordering.
- **V-2** Derived, never hand-maintained. `data:protocol-version` says so explicitly, and
  `rule:protocol-version-must-match` makes it a hard connection error, so a stale hand-written
  constant would take a fleet down rather than degrade.
- **V-3** **It covers wire-observable shape only.** This is the requirement most likely to be
  got wrong. Regenerating with a newer tinybind that emits identical bytes must produce the
  identical version, or every generator upgrade becomes a lockstep client and server redeploy
  (`rule:protocol-version-must-match` `operational_consequence`). Conversely, any change that
  moves one byte on the wire must move the version.
- **V-4** This is therefore a *different* hash from `rule:generation-input-hash`, which exists
  to decide whether to regenerate and legitimately covers things the wire never sees.
- **V-5** Stable across runs, platforms, and Go versions. A hash over a canonically serialized
  schema description, not over generated source text and not over anything map-ordered.
- **V-6** The generator should be able to emit the schema description itself, not only its
  hash, so that a version mismatch can be diagnosed by diffing two schemas rather than by
  observing that two opaque numbers differ.

---

## 8. Diff and patch generation

Not needed before Phase 3a, specified now because §4's traversal is what it is built on.

- **X-1** From the world-state struct set, emit a diff producing `data:state-delta` from a
  retained baseline and the current state, and a corresponding apply on the receiving side.
- **X-2** Traversal order is deterministic and identical on both sides. This falls out of
  D-5 as long as no map is ever ranged.
- **X-3** Entity creation and deletion are part of the delta, not only changed fields, per
  `data:state-delta` `content`.
- **X-4** The game declares struct types and field scales; it never hand-writes diff logic.
  That is the whole point of `decision:framework-side-delta-generation`.
- **X-5** The framework retains baselines and chooses one; the generator supplies only the
  diff and the apply. Retention cost, baseline choice, and ack tracking are ebigentserver's.
- **X-6** A delta encodes under either profile depending on size and schema stability
  (`data:state-delta` `encoding`), so the generated diff must be profile-agnostic and the
  profile chosen at the call site.
- **X-7** Naming: `rule:delta-consistency-model` already exists in tinybind for the HTML live
  boundary. A second, unrelated delta concept in the same knowledge base needs a distinct name
  from the start.

---

## 9. Acceptance

Per capability:

1. The generator emits a `PlayerInput` codec that produces the bytes pinned in
   `encoding/cbor/codec_test.go` `TestWireMessageEncodesToPinnedBytes`, and that hand-written
   file can be replaced by generated output with no test change.
2. Encode and decode of a fixed-shape wire message allocate zero in steady state, measured by
   `AllocsPerRun`, with a reused buffer and a reused `Reader`.
3. A struct holding a `[]Entity` whose fields include a foreign fixed-point type generates a
   complete codec — no field silently dropped (W-9).
4. A world-profile message with an unknown field decodes, skipping it; the same message under
   the wire profile is refused.
5. `float64` anywhere reachable from a world-state root is a generation error naming the type
   and the field — including behind a named scalar type (D-4).
6. A fixed-point field with no `scale` option is a generation error.
7. Regenerating with an unchanged schema produces an unchanged protocol version; changing one
   field's width changes it (V-3).
8. Generated code links and runs under TinyGo for `js/wasm`, per `requirement:tinygo-wasm`.
9. The same message encodes to identical bytes on `darwin/arm64`, `linux/amd64`, and
   `js/wasm` — the Phase 2 digest-equality property, reached through generated code.
10. A project generating no CBOR codec regenerates byte for byte, unchanged by any of this.

---

## 10. Open questions

1. **Naming.** `cborbind` follows `jsonbind`/`sqlbind`/`dynamobind`. But the two profiles are
   different enough — one is a frozen realtime format, the other an evolvable document format —
   that a single package name may be understating the split. One package with two declaration
   forms is assumed throughout; worth confirming before the API is published.
2. **Optional world-profile fields** (W-8). What an omitted field means, and whether tinybind's
   existing `omitempty`/`omitzero` tag vocabulary is reused or refused here as it is for
   foreign JSON fields.
3. **Foreign-append validation** (I-5). Always on, on under a build tag, or on only in the
   generated tests. Always-on costs a second pass over bytes the codec just wrote; off means an
   unverifiable contract on the determinism-critical path.
4. **Where the schema description lives** (V-6). A generated Go constant, a sidecar file, or
   both. A sidecar is diffable in review, which is where a protocol change should be caught.
5. Whether the generator emits the *tests* as well — pinned-bytes and round-trip tests per
   message type. `concept:future-generators` lists test generation as a future idea, and this
   is the case where the pinned bytes are the protocol, so an unpinned generated codec is a
   protocol with no record of what it used to be.
6. Whether `rule:generator-feature-disable` may disable the CBOR feature at all, given D-3.
