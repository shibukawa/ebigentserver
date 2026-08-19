---
id: rule:protocol-version-must-match
type: rule
title: Protocol Version Must Match Exactly
---
Mismatched data:protocol-version between peers is a hard connection error, not a negotiation.

```yaml
decided_behavior: reject connection with an explicit version error
rejected_alternative: per field compatibility negotiation, optional field fallback
rationale: concept:cbor-wire-profile has no field names, so partial compatibility is undetectable
check_point: handshake in flow:session-admission, before agent seating
operational_consequence: client and server deploy in lockstep, or run versioned endpoints
rollout: policy:protocol-rollout
replay: data:episode-log stores the version it was recorded under
```
