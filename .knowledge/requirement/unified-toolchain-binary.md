---
id: requirement:unified-toolchain-binary
type: requirement
title: Unified Toolchain Binary
---
One `ebigent` binary carries every development task; no per-task commands ship.

```yaml
binary: ebigent
absorbs:
  - behavior-editor: becomes the edit verb, folded into ui:dev-console
  - behavior-merge: becomes the merge verb
  - corpus-report: becomes the analyze verb
why: three independent flag sets drifted apart, none could locate a project root, and none shared configuration
gains: one install step, one version, one help tree, one project locator
surface: api:game-cli
decision: decision:one-ebigent-binary
extends: requirement:dedicated-game-toolchain
```
