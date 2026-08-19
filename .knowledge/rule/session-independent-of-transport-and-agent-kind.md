---
id: rule:session-independent-of-transport-and-agent-kind
type: rule
title: Session Independent Of Transport And Agent Kind
---
concept:session must not branch on transport type or agent implementation.

```yaml
allowed_dependencies: api:agent-interface, api:transport-interface
forbidden: protocol specific code paths, human versus bot special cases
enables: requirement:multi-execution-mode
```
