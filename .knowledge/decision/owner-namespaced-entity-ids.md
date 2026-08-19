---
id: decision:owner-namespaced-entity-ids
type: decision
title: Owner Namespaced Entity IDs
---
Entity IDs are namespaced by their creating authority, so allocation never needs coordination.

```yaml
decided: yes
scheme:
  - player created entities: slot id prefix plus a per slot counter
  - session spawned entities: a reserved session namespace plus counter
uniqueness: guaranteed by construction; two creators can never collide, so no allocation round trip exists
determinism: each counter advances only through simulation steps, so term:rollback resimulation reproduces identical ids
namespace_key: concept:player-slot id, not a player account id, since the slot is the stable in session identity
wire_form: small integers in concept:cbor-wire-profile; the prefix packs into high bits rather than a string
replay: ids regenerate identically from actions, which actor:replay-agent depends on
```
