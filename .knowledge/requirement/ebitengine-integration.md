---
id: requirement:ebitengine-integration
type: requirement
title: Ebitengine Integration
---
system:ebitengine is the first-class client target for rendering and human input.

```yaml
boundary: engine input never reaches game logic directly, see rule:no-engine-input-in-game-logic
adapter: api:input-adapter
render_input: concept:observation
priority: first target; other engines may follow but must not shape core abstractions
```
