---
id: decision:samples-as-test-infrastructure
type: decision
title: Samples Are Test Infrastructure
---
Sample games ship as the framework regression harness, not as demonstrations kept beside it.

```yaml
decided: yes
mechanism: ai vs ai matches drive real sessions through the real api:agent-interface, so no mock network client is needed
covers:
  - integration tests across transport, admission, and synchronization
  - load tests, by running many concurrent sessions
  - determinism tests, replaying recorded matches across architectures
  - replay corpus generation for flow:behavior-tree-synthesis
  - balance and regression checks through metric:balance-signals
runner: concept:training-farm and flow:automated-playtest, the same machinery games use
acceptance: concept:sample-acceptance-matrix fixes configuration and assertions for CI
consequence: a framework change that breaks a sample breaks the build, so samples cannot rot
cost: samples must stay maintained at library quality, which is the point
```
