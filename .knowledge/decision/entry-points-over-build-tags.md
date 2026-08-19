---
id: decision:entry-points-over-build-tags
type: decision
title: Entry Points Over Build Tags
---
Build variants are separate main packages, not build tags scattered through the library.

```yaml
decided: yes
mechanism: one cmd entry point per concept:build-target, each importing only what it needs
why_it_works: the Go linker excludes an unimported package, so a dedicated server drops system:ebitengine by never importing it, with no tag anywhere
rejected_alternative: a headless tag threaded through library packages
rejection_reasons:
  - every tagged file needs a matching stub, doubling the surface that can drift
  - tag combinations multiply, and only some combinations are ever compiled
  - a missing tag fails at link time in a distant package instead of at the entry point
remaining_tag_uses: rule:build-tag-only-for-linkage
verification: rule:engine-import-confined-to-client-entry checks the import graph rather than trusting convention
```
