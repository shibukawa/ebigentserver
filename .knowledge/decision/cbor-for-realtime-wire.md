---
id: decision:cbor-for-realtime-wire
type: decision
title: CBOR For Realtime Wire Format
---
Use CBOR rather than JSON or a bespoke binary format for realtime messages.

```yaml
decided: yes
rationale:
  - smaller than json
  - data model close to json, so existing type analysis is reusable
  - two profiles cover both compact and evolvable needs
profiles: concept:cbor-wire-profile, concept:cbor-world-profile
browser_json_rejected:
  - native JSON.parse speed is unreachable from a go wasm client: the parsed value is a js object, and reading it back into go crosses the wasm boundary once per field, which costs more than the parse saves
  - a go wasm client therefore parses in go either way, so the native argument does not apply and cbor decodes from linear memory without crossing at all
  - at realtime message sizes the parse cost is negligible for a hand written js client too; native json only leads on payloads far larger than a tick message, where cbor being smaller offsets it
  - json numbers are ieee754 doubles in js, so a scaled int64 of rule:fixed-point-on-wire rounds silently past 2^53 and breaks determinism without an error
  - a second encoding would fork data:game-version, which is derived from one generated schema
json_still_used: debug output, control plane api, and data:episode-log, see rule:profile-selection-by-message-kind
serves: requirement:compact-binary-protocol
```
