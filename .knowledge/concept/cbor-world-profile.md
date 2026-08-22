---
id: concept:cbor-world-profile
type: concept
title: CBOR World Profile
---
Full CBOR feature set for large or evolving structures.

```yaml
allowed: map, array, optional fields, nested structures, tags, extensible schema
numerics: still scaled integers for simulation fields, see decision:fixed-point-numeric-representation
applies_to: data:snapshot, data:episode-log
tradeoff: larger payload, tolerant of schema change
version_check: still gated by rule:game-version-must-match for live connections
selection: rule:profile-selection-by-message-kind
```
