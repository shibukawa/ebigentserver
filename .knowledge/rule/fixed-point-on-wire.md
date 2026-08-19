---
id: rule:fixed-point-on-wire
type: rule
title: Fixed Point On Wire
---
Deterministic message payloads carry integers only; float is rejected in concept:cbor-wire-profile.

```yaml
allowed: sized integers with declared scale, bool, bytes, string, nested array
forbidden: float, unscaled real numbers
scale_source: schema declaration, not runtime negotiation
mismatch: scale change is a protocol change, see rule:protocol-version-must-match
decision: decision:fixed-point-numeric-representation
```
