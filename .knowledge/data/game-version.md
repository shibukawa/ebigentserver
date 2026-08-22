---
id: data:game-version
type: data
title: Game Version
---
What two players must hold in common to play together: a fingerprint derived from the game's own generated schema, compared once.

```yaml
named_for_what_decides_it: >
  the game's types decide the value, so the game is what it versions. Calling
  it a protocol version borrows the vocabulary of HTTP/1.1 — an ordered,
  negotiated spec revision — which is the opposite of what this is, and an
  Ebitengine developer reading the field imports that reading. "Different game
  version, cannot join" is the sentence a player already understands, and it
  is exactly what a mismatch means.
not_the_release_version: >
  a patch touching no wire type keeps the same value, and a point release that
  adds one state field changes it, so it tracks the schema rather than the
  number on the box. It must stay derived: a hand-written one is forgotten at
  exactly the release that needed it, and the failure is silent. A declared
  string type does not prevent that — Go assigns an untyped constant to one —
  so the field wants a type a literal cannot spell, [8]byte or a struct with
  unexported fields, matching how this repository already declares Kind, Mode,
  and DiffClass.
not_ordered: >
  the value is a hash — "787d6000386004c1" in the tutorial — derived from the
  schema and regenerated with it. Nobody bumps it, there is no ordering, no
  range, and no negotiation, so the versioning vocabulary around it overstates
  what it is. It is a build identity, and the only question ever asked of it
  is whether two of them are equal.
covers: every codec the build links, across every stage of decision:codec-set-per-stage
generation: derived from generated schema by system:tinybind, not hand maintained
compared_at:
  - the discovery beacon, where a listener drops a beacon whose fingerprint differs, so a mismatched host is hidden rather than shown and broken
  - the seat request, which answers 409 rather than granting
  - the admission handshake, before seating, per rule:game-version-must-match
why_it_cannot_be_dropped: >
  the comparison is what makes two peers matched, not a check performed after
  they already are. Without it two builds of one game find each other and
  connect; concept:cbor-wire-profile encodes fixed-order arrays with no field
  names, so the resulting misdecode produces plausible garbage rather than an
  error.
not_carried_per_message: >
  within a match the wire needs only a stage index. Both ends hold the same
  stage list in the same order once the fingerprint matched, so the index
  selects the codec by construction.
rollout: policy:protocol-rollout, whose versioned endpoints and staged drain apply to a hosted deployment; the direct pair case of concept:trust-model reduces the whole of it to hiding a mismatched beacon
replay: data:episode-log stores the fingerprint it was recorded under
```
