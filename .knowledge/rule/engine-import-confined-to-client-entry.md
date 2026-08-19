---
id: rule:engine-import-confined-to-client-entry
type: rule
title: Engine Import Confined To Client Entry Points
---
Only client entry points may import system:ebitengine; game rules and session packages may not.

```yaml
enforced_by: an import graph check in the same generation pass as rule:codegen-rejects-nondeterministic-types
failure: build error naming the offending package and import path
why_structural: this is the compile time form of rule:no-engine-input-in-game-logic, which otherwise holds only by convention
consequence: a headless target cannot accidentally acquire a rendering dependency through an unrelated package
same_check_covers: cgo dependencies, which would break the wasm target of requirement:native-and-wasm-targets
```
