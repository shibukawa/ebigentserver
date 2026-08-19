---
id: data:protocol-version
type: data
title: Protocol Version ID
---
Single identifier covering message schema, field order, and fixed point scales.

```yaml
fields:
  - protocol_version_id
sent_at: connection handshake, before any concept:action or state message
covers: concept:cbor-wire-profile schemas, concept:cbor-world-profile schemas, scale factors
generation: derived from generated schema by system:tinybind, not hand maintained
rule: rule:protocol-version-must-match
deployment: policy:protocol-rollout
```
