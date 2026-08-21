---
id: rule:debug-endpoint-excluded-from-release
type: rule
title: Debug Endpoint Excluded From Release
---
api:dev-debug-endpoint must not link into a release artifact.

```yaml
mechanism: a development entry point imports it and a release entry point does not, per decision:entry-points-over-build-tags
why: the endpoint exposes full concept:world-state, which defeats policy:observation-scoped-information and rule:analysis-restricted-to-visible-fields for anyone who reaches it
not_sufficient: binding to loopback, or gating on a flag — the code must be absent, not merely unreachable
verification: an import graph check in the manner of rule:engine-import-confined-to-client-entry
serves: requirement:production-runtime-safety
```
