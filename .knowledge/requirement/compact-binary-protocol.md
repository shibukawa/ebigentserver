---
id: requirement:compact-binary-protocol
type: requirement
title: Compact Binary Wire Protocol
---
Realtime messages must use a binary encoding smaller than JSON.

```yaml
encoding: cbor
profiles:
  - concept:cbor-wire-profile
  - concept:cbor-world-profile
selection: rule:profile-selection-by-message-kind
codegen: system:tinybind, see decision:reuse-tinybind-codegen
```
