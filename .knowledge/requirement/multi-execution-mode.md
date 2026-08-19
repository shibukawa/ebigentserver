---
id: requirement:multi-execution-mode
type: requirement
title: Multi Execution Mode Support
---
One session implementation must run under all supported execution modes without code forks.

```yaml
modes:
  - concept:standalone-mode
  - concept:listen-server-mode
  - concept:dedicated-server-mode
  - concept:simulation-mode
selection: build target plus configuration, see concept:build-target
constraint: rule:session-independent-of-transport-and-agent-kind
```
