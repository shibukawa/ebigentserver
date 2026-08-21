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
migration: the old binaries stay buildable until the samples and README reference the verbs instead
serves: requirement:unified-toolchain-binary
```
