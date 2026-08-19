---
id: requirement:sample-suite-proves-capabilities
type: requirement
title: Sample Suite Proves Capabilities
---
Sample games exist to demonstrate architectures, not genres; each one proves a framework capability works.

```yaml
not_a_goal: a varied collection of games
goal: every axis of concept:coverage-matrix is exercised by at least one running sample
acceptance: concept:sample-acceptance-matrix; optional or undecided variants do not count
ordering: concept:sample-progression, each step adding one capability
constraint: rule:sample-adds-one-capability
double_duty: decision:samples-as-test-infrastructure makes the suite the regression harness too
failure_signal: an axis with no sample is an untested claim in this catalog
```
