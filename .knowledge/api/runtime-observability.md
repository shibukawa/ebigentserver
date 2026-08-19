---
id: api:runtime-observability
type: api
title: Runtime Observability
---
Stable operational signals exposing health, capacity, lifecycle, network quality, and deterministic failures.

```yaml
health:
  liveness: process event loop responds
  readiness: configuration and keys valid; admission capacity available
metrics:
  - active sessions and connections
  - tick duration and missed tick budget
  - queue, reassembly, snapshot, and retained baseline bytes
  - input loss, rtt, rejected admission, disconnect, and overload counts
  - replay checkpoint mismatches
logs:
  required_context: session id, connection id, tick, protocol version, lifecycle state, reason code
  forbidden: raw data:session-ticket, private key, and full player observation by default
events: session start, drain, end, abort, admission reject, abuse reject, and data:state-checkpoint mismatch
cardinality: player and session identifiers belong in logs; metrics use bounded labels
```
