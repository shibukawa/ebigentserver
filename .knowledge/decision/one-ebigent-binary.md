---
id: decision:one-ebigent-binary
type: decision
title: One Ebigent Binary
---
Ship one `ebigent` command instead of three separate tool binaries.

```yaml
decided: yes
absorbed: behavior-editor, behavior-merge, corpus-report
mechanism: one configbind SubCommand per verb, so options, positionals, and usage text are generated from a struct rather than hand-written flag sets
still_true: decision:separate-game-cli — ebigent remains separate from the system:popcornweb pw command
migration:
  - corpus-report and behavior-merge are gone; their logic was already in the analysis and behavior packages, so the verbs call the same code
  - behavior-editor stays a separate command until edit can open ui:dev-console, since decision:single-dev-console-ui makes its final form depend on the console
serves: requirement:unified-toolchain-binary
```
