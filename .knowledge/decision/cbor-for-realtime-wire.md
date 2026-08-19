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
serves: requirement:compact-binary-protocol
```
