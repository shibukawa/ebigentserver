# cbor — Requirements

Move `popcornweb/contrib/cbor` into tinygodriver, and extend it from a WebAuthn/COSE
reader into the codec that carries `concept:cbor-wire-profile` and
`concept:cbor-world-profile` (`plan.md` Phase 0).

- Source: `github.com/shibukawa/popcornweb/contrib/cbor` (petitweb-go working copy)
- Destination module: `github.com/shibukawa/tinygodriver` (currently v1.2.4)
- Proposed import path: `github.com/shibukawa/tinygodriver/cbor`
- License: Apache-2.0 on both sides, so the move raises no licensing question
- Status: **specification** — no code moved yet

---

## 1. Purpose

### 1.1 Why it moves

The package sits in `contrib/` of an application framework, but nothing about it is
web-framework-shaped: it is a bounded, reflection-free, TinyGo-safe binary codec.
That is exactly the category tinygodriver already collects (`jwt`, `websocket`,
`httpmux`, the SigV4 signer), and `decision:package-layout` puts such packages flat
at the repository root.

Three consumers are now in play, and only one of them is popcornweb:

| Consumer | Uses | Profile |
|---|---|---|
| `popcornweb/contrib/passkey` | attestation objects, COSE_Key | CTAP2 canonical CBOR |
| ebigentserver realtime path | `data:player-input`, `data:game-event`, `data:state-delta` | `concept:cbor-wire-profile` |
| ebigentserver world path | `data:snapshot`, `data:episode-log` | `concept:cbor-world-profile` |

Leaving the codec inside popcornweb would make the game framework depend on a web
framework for its wire format, which `decision:independent-from-popcorn-wave` rules out.

The dependency direction already works: popcornweb requires tinygodriver v1.2.4 today,
so consuming the package from its new home adds no edge to the graph. **tinygodriver
must never import popcornweb**, which the move must preserve.

### 1.2 In scope

- Relocating the package, its tests, and its consumers (§3)
- A published codec interface so a type this package never analyzed can encode itself (§5.1)
- Fixed-point / scaled-integer support built on that interface (§5.2)
- Two enforceable encoding profiles (§5.3)
- Allocation and throughput work required by a 60 Hz tick loop (§5.4)
- Corrections to behaviour that is wrong, mis-documented, or reflection-backed today (§5.5)

### 1.3 Out of scope

| Concern | Owner |
|---|---|
| Struct analysis, schema derivation, generated encode/decode | `system:tinybind` |
| Rejecting non-deterministic struct fields at build time | `rule:codegen-rejects-nondeterministic-types` |
| `data:protocol-version` derivation and the handshake check | `system:tinybind`, `flow:session-admission` |
| Fixed-point arithmetic itself | `github.com/shibukawa/fixmath` |
| Message framing, backpressure, sequence/ack | `api:message-framing`, `api:sequence-ack-layer` |
| Choosing a profile per message kind | `rule:profile-selection-by-message-kind`, the caller |
| Delta and patch generation | `decision:framework-side-delta-generation` |
| COSE signing/encryption, CBOR diagnostic notation | nobody; still refused |

This package supplies primitives and enforcement. It never decides what a message means.

### 1.4 Normative references

- `concept:cbor-wire-profile`, `concept:cbor-world-profile`, `decision:cbor-for-realtime-wire`
- `rule:fixed-point-on-wire`, `rule:no-float-in-simulation`, `rule:profile-selection-by-message-kind`
- `rule:protocol-version-must-match`, `data:protocol-version`
- `decision:fixed-point-numeric-representation`, `api:fixed-point-math`, fixmath REQUIREMENTS.md
- tinygodriver `decision:package-layout`, `decision:jwt-package-reuse`
- tinybind-go `requirement:json-codec-interface` — the settled precedent for §5.1
- RFC 8949, in particular §4.2.1 (core deterministic) and §4.2.3 (length-first ordering)

---

## 2. What is being moved

1,916 lines across ten files, of which 1,451 are non-test.

| File | Lines | Contents |
|---|---|---|
| `decoder.go` | 864 | incremental `io.Reader` decoder, token stream, typed reads, `ReadRaw`, `Validate` |
| `encoder.go` | 437 | deterministic writer, `Marshal*` raw constructors, a validating parser |
| `types.go` | 138 | options, errors, `Token`, `RawMessage`, `MapEntry` |
| `doc.go` | 10 | package doc |
| tests | 467 | RFC 8949 Appendix A vectors, allocation bounds, fuzz (`!tinygo`), examples |

What it already does well, and must not regress:

- No reflection in the decode path, no `io.ReadAll`, no struct mapping
- Bounded everything: input bytes, nesting, container items, string bytes, retained raw
- A declared length never becomes an allocation — the anti-amplification property that
  `allocation_test.go` pins, added because this decoder reads unauthenticated attestations
- Optional duplicate-map-key rejection, validating a bounded root before exposing tokens
- Explicit sequence mode; otherwise trailing bytes after one root item are an error
- Refuses indefinite-length *output*, because it cannot be deterministic

What it does not do at all, and §5 is about:

- Any mapping between CBOR and a Go value. The API is tokens and raw items only.
- Any way for a type to say how it encodes. There is no interface in the package.
- Any notion of a scale, a profile, or a rejected float.
- Any allocation-free path. Every primitive write allocates; every string read allocates.

---

## 3. The move

- **M-1** New home is `github.com/shibukawa/tinygodriver/cbor`, package `cbor`, flat at the
  repository root beside `jwt` and `httpmux`, per `decision:package-layout`. tinygodriver has
  no `contrib/` tree and should not grow one for this.
- **M-2** tinygodriver gains no module dependency. The package imports only
  `bytes`, `encoding/binary`, `fmt`, `io`, `math`, `sort`, `strconv`, `unicode/utf8` today,
  and §5.5 removes `sort`.
- **M-3 This is a move, not a vendoring.** `decision:jwt-package-reuse` copied `jwt` and
  documented a deliberate divergence, and both copies exist today. That trade is wrong here.
  A signer that diverges produces a token one side rejects loudly; **a codec that diverges
  produces bytes both sides accept and read differently**, which surfaces as a desync rather
  than an error. There must be exactly one implementation.
- **M-4** `popcornweb/contrib/cbor` is deleted and its consumers re-imported:
  `contrib/passkey/parse.go:14`, `contrib/passkey/passkeytest/encode.go:10`,
  `contrib/passkey/passkey_test.go:22`, `contrib/cbor/example_test.go:7`.
  A type-alias shim at the old path is acceptable for one popcornweb release if external
  callers exist; it must be marked deprecated and removed on a named schedule.
- **M-5** Tests move with the package, unchanged. The allocation bound, the Appendix A
  vectors, and the `!tinygo`-tagged fuzz target are the regression evidence for everything
  §5 then changes.
- **M-6** The WebAuthn contract is frozen at move time. passkey is the one production
  consumer, and it must pass on the moved package before any §5 work starts.
- **M-7** Package documentation is rewritten for the new consumer set. `README.md` and
  `doc.go` currently say the package is "designed for WebAuthn authenticator data and COSE
  keys"; after the move it is equally a game-wire codec, and §5.5 C-1 shows that the
  WebAuthn framing has already caused one documentation error.

---

## 4. The determinism contract

Everything in this section is a property the wire profile must hold, not a guideline.
It exists because `plan.md` Phase 2 gates on replaying an episode on arm64 and amd64 and
getting identical digests, and this codec is on that path.

- **D-1** The same value encodes to the same bytes on every architecture, every run, forever.
  A change to the encoding is a `data:protocol-version` change (`rule:protocol-version-must-match`).
- **D-2** No float reaches the wire profile, at encode or at decode (`rule:fixed-point-on-wire`).
  This is enforced, not documented — see FP-5.
- **D-3** No map iteration order is ever observable in output. Deterministic map output already
  sorts; the requirement is that nothing new introduces an unsorted path.
- **D-4** `int`-width assumptions must be audited. `decoder.go:61`, `:103`, `:120`, and `:377`
  compare and convert against `int` / `math.MaxInt`, which is 64-bit on the dedicated server
  and 32-bit on a `js/wasm` client. fixmath bans `int` outright (its D-4) for this reason;
  this package cannot, since `len` is `int`, so instead every such site must be shown to
  reach the same decision on both widths, or be rewritten in terms of `int64`/`uint64`.
- **D-5** Encoded output is byte-identical between a TinyGo build and a standard Go build.

---

## 5. Functional requirements

### 5.1 A published codec interface

This is the "an interface like `MarshalJSON`" requirement. The **role** is exactly
`MarshalJSON`'s — a type carries its own encoding, and the codec dispatches to it rather than
deriving one. The **shape** should not be `MarshalJSON`'s, for a reason already settled next
door.

- **A-1** The package declares its own interfaces. Proposed:

  ```go
  type Appender interface { AppendCBORTo(dst []byte) []byte }
  type Decoder  interface { DecodeCBORFrom(data []byte) error }  // pointer receiver
  ```

- **A-2** Append-into-destination, not byte-returning. tinybind-go settled this on 2026-08-13
  in `requirement:json-codec-interface` and named the pair `AppendJSONTo` / `DecodeJSONFrom`.
  Its reasoning transfers unchanged: a `Marshal() ([]byte, error)` allocates once per value and
  undoes the caller's buffer pooling at every nested field, while `encoding/json/v2` moved to
  writing into an encoder for the same reason. In a 60 Hz loop the difference is not stylistic.
  Names should match tinybind's pair so the two codecs read alike.
- **A-3** `AppendCBORTo` returns no error, matching `AppendJSONTo`. The append path carries no
  error below this point, and adding one would restructure every emitted encoder. The doc
  comment must state the resulting obligation: the implementation appends a valid, complete,
  single CBOR item for every value of its type.
- **A-4** Interface dispatch is a type assertion, not field walking, so `decision:reflection-free`
  and every TinyGo target are unaffected.
- **A-5** Precedence: the method wins over any generated or analyzed encoding for the same type.
  `encoding/json` resolves the same conflict the same way. Generating a codec for a type whose
  author wrote one, and then using the generated one, silently emits bytes the author did not
  intend.
- **A-6** tinybind must recognize the interface structurally (via `go/types`, as its JSON side
  already does for `KindForeign`), so a field of a foreign type is admitted, resolved at
  generation time, and emitted as one named call with no runtime branch.
- **A-7** Composition at depth. Encoding composes for free, since the method appends into the
  destination the parent is already building. Decoding at depth needs a captured sub-item;
  `ReadRaw` is already the bounded capture primitive, so the requirement is that it stays
  bounded by `MaxRawMessageBytes` when used this way.
- **A-8 The contract is unverifiable, and here that is not acceptable on its own.** tinybind's
  JSON side accepts that a hand-written method opts out of the module's tag semantics. For CBOR
  the stakes differ: a foreign method could append a float, an indefinite-length item, or an
  unsorted map into a wire-profile message, and D-1/D-2 would be silently broken. Therefore the
  wire-profile encoder must **validate the bytes a foreign method appended** against the profile
  (the `deterministicParser` in `encoder.go` already does exactly this job for `WriteRaw`), at
  least under a build tag or an option that CI turns on.
- **A-9 Open, and worth deciding before publishing:** whether to also recognize the de-facto
  ecosystem pair `MarshalCBOR() ([]byte, error)` / `UnmarshalCBOR([]byte) error`
  (fxamacker/cbor's spelling, and the literal answer to "like `MarshalJSON`"). Recommendation:
  declare the append pair as primary and recognize the allocating pair as a secondary arm, in
  that precedence order — own interface first, then the standard one — which is the order
  tinybind chose for the same question.

### 5.2 Fixed point

- **FP-1** Wire-profile numerics are bare CBOR integers. The scale lives in the schema and is
  covered by `data:protocol-version`; it is never transmitted and never negotiated
  (`rule:fixed-point-on-wire`).
- **FP-2** A fixmath value must be encodable without either package importing the other.
  `fixmath.F64` is a defined type over `int64`; the §5.1 interface is the seam that lets it
  carry its own encoding. **cbor must not depend on fixmath** — fixmath is spec-only today, and
  a codec that depends on one math library cannot serve a game that supplies its own core
  (`api:fixed-point-math` substitution clause).
- **FP-3** Scale conversion — declared wire scale to and from fixmath's canonical 2⁻³² — belongs
  to fixmath and to generated code, not here. cbor supplies range-checked sized-integer
  primitives and nothing else.
- **FP-4** Range-checked typed reads. The encoder writes the shortest form, so a field declared
  as `int32` arrives as anything from one to five bytes. The decoder must be able to enforce the
  declared width: `ReadInt32`, `ReadUint16`, and so on, where an out-of-range value is a protocol
  error rather than a silent wrap. Today only `ReadInt`/`ReadUint` exist, and `ReadInt`
  overflows to `ErrIntegerOverflow` at `int64` only.
- **FP-5** Float is *refused*, not merely unused, under the wire profile. Today `WriteFloat`,
  `MarshalFloat`, and float decoding are unconditionally available. Under the wire profile
  every one of them must be an error, on both sides, so that a float leak is a caught protocol
  violation rather than a desync (`rule:no-float-in-simulation` enforcement_property).
- **FP-6 Open:** whether the world profile may carry CBOR tag 4 (decimal fraction,
  `[exponent, mantissa]`) to make a scaled value self-describing. Recommendation: no. Tag 4's
  exponent is base-10 while these scales are binary, and `data:protocol-version` already covers
  what the tag would carry. Recording the refusal is worth more than the option.

### 5.3 Profiles

- **P-1** Two named option presets, one per profile concept, so a caller names a profile rather
  than assembling limits by hand.
- **P-2** Wire profile (`concept:cbor-wire-profile`) enforces: definite lengths only; arrays
  only, no maps; no floats (FP-5); no tags; no text keys; fixed item count and order per message
  schema; a small nesting bound. Field names never appear — this is what makes partial
  version compatibility undetectable and is why `rule:protocol-version-must-match` is a hard
  error rather than a negotiation.
- **P-3** World profile (`concept:cbor-world-profile`) enforces: deterministic map key order,
  no indefinite-length output, optional fields and tags permitted, larger bounds for snapshots.
- **P-4** `Validate(data, opts)` gains a profile, so "is this byte string legal under profile X"
  is answerable without decoding it into anything.
- **P-5** Profile *selection* stays with the caller and with codegen
  (`rule:profile-selection-by-message-kind`). This package enforces a profile it is handed; it
  never infers one from a message.

### 5.4 Allocation and throughput

The package was written for one attestation per login. It is now proposed for every input of
every player of every tick, and the current shape does not survive that.

- **PF-1** Byte-slice entry points on both sides: encode by appending into a caller-owned
  `[]byte`, decode from a `[]byte` with no `io.Reader` wrapping. The `io.Reader` decoder stays
  for the passkey path and for episode logs.
- **PF-2** `appendHead(nil, …)` allocates on every primitive write (`encoder.go`, all
  `Write*`/`Marshal*` entries). Every one must take a destination slice.
- **PF-3** Reusable codec objects — `Reset(dst []byte)` and `Reset(data []byte)` — so a session
  keeps one encoder and one decoder per connection instead of one per message.
- **PF-4** Steady-state zero allocation for a fixed-shape wire message, gated by a
  `testing.AllocsPerRun` test. The package already has the precedent for pinning an allocation
  property in a test rather than a comment.
- **PF-5** `Token.Bytes` and `Token.Text` allocate per string. The byte-slice decoder should
  offer a borrow-from-input mode with a documented lifetime; the reader decoder keeps copying.
- **PF-6** Benchmarks for both profiles, on both compilers, as the evidence that PF-1..PF-5
  actually paid.

### 5.5 Corrections to current behaviour

These are defects and mis-statements found while reading the package. They are worth fixing
during the move, while there is exactly one consumer to re-verify against.

- **C-1 The map key ordering is not what the documentation says.** `encoder.go:170` sorts
  shorter keys first and then bytewise, and the validating parser enforces the same. That is
  RFC 8949 §4.2.3 *length-first* ordering — RFC 7049 canonical CBOR, which is what CTAP2 and
  COSE require, so the behaviour is right for passkey. But `README.md` and `doc.go` both claim
  "RFC 8949 Core Deterministic Encoding", and §4.2.1 core deterministic sorts **bytewise
  lexicographically on the encoded key, without a length pass**. The two orders differ.
  Requirement: make the ordering an explicit, selectable property — length-first for
  CTAP2/COSE, bytewise for core deterministic — default it per profile, correct the
  documentation, and pin both orders with a test whose vectors disagree.
- **C-2 The package is not reflection-free, contrary to its own package doc.** `sort.Slice`
  (`encoder.go:170`) and `sort.Strings` (`decoder.go:833`) go through the reflect-based swapper.
  Replace with `slices.SortFunc` / `slices.Sort`, which are generic, allocation-free, and
  TinyGo-clean. The module floor is Go 1.26, so `slices` is available.
- **C-3 Duplicate-key detection is O(n²) in allocation on the untrusted path.**
  `keyFingerprint` (`decoder.go:770-855`) builds Go strings by repeated `+=` concatenation, per
  key, per nested item. This runs under `RejectDuplicateMapKeys`, which is the mode recommended
  for untrusted input. Replace with a bounded scheme — a hash set over the captured raw key
  sub-slices — staying inside `MaxRawMessageBytes`.
- **C-4 `readBytes` reads one byte at a time.** `decoder.go:102-132` calls `d.readByte()` in a
  loop, and each call is an `io.ReadFull` on a one-byte array; `readChunkBytes` is used only as
  an initial capacity. Read in chunks, while preserving the property
  `TestADeclaredLengthDoesNotBecomeAnAllocation` proves: a declared length must still never be
  reserved before its bytes arrive.
- **C-5** Document and bound the retention cost of `RejectDuplicateMapKeys`, which may hold a
  whole root item up to `MaxRawMessageBytes`. Defaults are 1 MiB; a `data:snapshot` under the
  world profile can exceed that, so the world preset must set it deliberately.

---

## 6. Non-functional requirements

- **N-1** One source builds and behaves identically under TinyGo and standard Go, which is the
  repository's stated premise. The `!tinygo` fuzz tag is the only permitted divergence.
- **N-2** Platform matrix must include `js/wasm` and one 32-bit target alongside the server
  targets, because of D-4. This is the cheapest way to catch the `int`-width assumptions before
  a browser client and a native server disagree about a length bound.
- **N-3** No new module dependencies, no cgo, no build-tag variants that change encoded bytes.
  Build tags may select an implementation; they may never select a wire format.
- **N-4** Errors stay classified as they are today (`ErrMalformed`, `ErrTruncated`,
  `ErrLimitExceeded`, `ErrDuplicateMapKey`, `ErrExtraneousData`, `ErrUnexpectedToken`,
  `ErrIntegerOverflow`), and a limit refusal keeps naming the limit it hit. Any new refusal —
  profile violation, range violation, foreign-method violation — gets its own sentinel.

---

## 7. Acceptance

The move (§3) is done when:

1. `popcornweb/contrib/passkey` passes its existing tests against `tinygodriver/cbor`, with no
   change beyond the import path.
2. The RFC 8949 Appendix A vectors, the allocation bound, and the fuzz target pass in the new
   home.
3. `popcornweb/contrib/cbor` no longer exists, or exists only as a deprecated alias with a
   removal date.
4. tinygodriver still imports nothing from popcornweb.

The extension (§5) is done when:

5. A `fixmath`-shaped type encodes and decodes through the §5.1 interface, with no dependency
   in either direction between `cbor` and `fixmath`.
6. tinybind generates a wire-profile codec for a struct containing such a field, and the
   generated call names one path with no runtime type switch.
7. A wire-profile message round-trips byte-identically on `darwin/arm64`, `linux/amd64`, and
   `js/wasm`.
8. A float in a wire-profile message is an error at encode **and** at decode, including one
   appended by a foreign `AppendCBORTo` (A-8).
9. A fixed-shape wire message encodes and decodes with zero allocations in steady state.
10. Both orderings of C-1 are pinned by tests whose expected bytes differ, and the
    documentation matches the default each profile selects.
11. A TinyGo build of both consumers links and runs.

---

## 8. Open questions

1. **A-9** — whether `MarshalCBOR`/`UnmarshalCBOR` is recognized as a secondary arm, or refused.
2. **FP-6** — tag 4 in the world profile: recommended refusal, not yet decided.
3. Whether the generated codec is owned by tinybind (generator) with tinygodriver supplying only
   the runtime, mirroring the `jsonbind` split. This is assumed throughout §5 but not decided.
4. Whether `RawMessage` should gain a profile-tagged variant, so a raw item validated under the
   world profile cannot be spliced into a wire-profile message by mistake.
5. Package placement: flat `cbor` at the root, as assumed by M-1, versus a `codec/` tree if a
   second binary format ever arrives.
6. Whether the world profile needs an indefinite-length *output* mode for streaming episode
   logs, which D-1 currently forbids and which `data:episode-log` may want.
