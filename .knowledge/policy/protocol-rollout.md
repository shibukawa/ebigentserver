---
id: policy:protocol-rollout
type: policy
title: Protocol Rollout
---
Deployment policy preserving exact wire version matching while clients, servers, tickets, and replays overlap in time.

```yaml
endpoint:
  - each active data:protocol-version has a distinct routable endpoint
  - data:session-ticket audience and endpoint select that version
rollout:
  - deploy and ready the new server endpoint
  - issue new tickets only after readiness
  - update clients
  - stop old ticket issuance, drain old sessions, then remove the old endpoint
rollback: resume ticket issuance to the still ready prior endpoint; never negotiate fields in session
replay:
  - raw data:episode-log remains immutable and names its version
  - replay uses matching generated schema or an explicit offline migration
  - migration creates a new artifact and retains source evidence
key_failure: unknown signing kid fails admission; refresh keys before readiness expires
```
