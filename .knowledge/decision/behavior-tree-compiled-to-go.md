---
id: decision:behavior-tree-compiled-to-go
type: decision
title: Behavior Tree Compiled To Go
---
An approved data:behavior-tree and its data:derived-predicate set are generated as Go source, not interpreted at runtime.

```yaml
decided: yes
rejected_alternative: a tree interpreter walking a data structure each tick
rejection_reasons:
  - per tick interpretation costs more than the decision itself in small games
  - generated code is readable, diffable, and reviewable in the same way as hand written agents
  - the compiler checks predicate signatures and observation field names, so a stale predicate fails the build
generator: system:tinybind, the same pass that already reads the observation types
output_is_source: a developer may edit generated code, but the edit belongs upstream in the tree or predicate, see rule:regeneration-preserves-approved-nodes
runtime: actor:behavior-tree-agent is then ordinary Go with no reflection
```
