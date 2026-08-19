---
id: rule:sample-adds-one-capability
type: rule
title: Sample Adds One Capability
---
Each sample introduces exactly one new framework capability and is the smallest game that can demonstrate it.

```yaml
rationale: a sample that adds three capabilities cannot localize a regression to one of them
selection_test: remove the new capability and the sample becomes a duplicate of an earlier one
size_test: any rule the game adds beyond the capability under test is decoration
ordering: the capability must build on the ones already proven, see concept:sample-progression
documentation: each sample names the concepts it exercises, so the catalog and the code stay linked
```
