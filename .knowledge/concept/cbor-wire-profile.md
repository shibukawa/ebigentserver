---
id: concept:cbor-wire-profile
type: concept
title: CBOR Wire Profile
---
Restricted CBOR subset for realtime messages, encoding structs as fixed-order arrays without field names.

```yaml
allowed: array, sized integer, bool, bytes, string, nested array
forbidden: float, map keys, optional fields, runtime schema change
numerics: scaled integers only, see rule:fixed-point-on-wire
required: fixed schema, fixed field order, one data:protocol-version for all peers
example: PlayerInput{Tick,MoveX,MoveY,Buttons} encodes as [1234,-1,0,3]
applies_to: data:player-input, data:game-event
codegen: system:tinybind
```
