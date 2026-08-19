---
id: data:player-input
type: data
title: Player Input Message
---
Upstream per-tick action message, the smallest and most frequent wire payload.

```yaml
fields:
  - name: tick
    type: uint32
  - name: move_x
    type: int8
  - name: move_y
    type: int8
  - name: buttons
    type: uint8
encoding: concept:cbor-wire-profile
example: [1234, -1, 0, 3]
carries: concept:action
```
