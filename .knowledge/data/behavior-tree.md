---
id: data:behavior-tree
type: data
title: Behavior Tree Artifact
---
The approved, versioned tree that actor:behavior-tree-agent executes.

```yaml
contents:
  - nodes with condition, action, and priority
  - the data:derived-predicate set the conditions are written over
  - per node level gates, see concept:skill-level-gating
  - provenance per node: authored by hand, or accepted from a data:behavior-candidate
  - the concept:behavior-evidence reference retained for accepted nodes
source_of_truth: this artifact, not the analyzer that proposed it
compilation: decision:behavior-tree-compiled-to-go emits Go source from it
editing: by hand or through ui:behavior-tree-editor, both valid
regeneration: rule:regeneration-preserves-approved-nodes
runtime_pairing: gates decide what the agent knows; concept:behavior-profile decides how well it executes
```
