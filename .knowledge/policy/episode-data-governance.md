---
id: policy:episode-data-governance
type: policy
title: Episode Data Governance
---
Operator-declared retention and access rules govern recorded observations, actions, identities, and training exports.

```yaml
classification:
  - player identity and free form agent output are sensitive
  - observations may reveal private role or team information
requirements:
  - declare retention duration and deletion process
  - restrict raw log and training corpus access
  - pseudonymize player identifiers before analysis unless identity is required
  - preserve concept:visibility-scope when producing derived datasets
  - record consent or another operator approved basis before human play trains behavior
logging_boundary: api:runtime-observability excludes raw credentials and observations by default
applies_to: data:episode-log, flow:behavior-tree-synthesis, flow:behavior-profile-derivation
```
