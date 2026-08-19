---
id: rule:no-engine-input-in-game-logic
type: rule
title: No Engine Input In Game Logic
---
Game logic must accept only concept:action, never system:ebitengine device state.

```yaml
enforced_at: api:input-adapter boundary
rationale: engine types are unavailable in headless and simulation builds
violation_effect: bots, replay, and dedicated server builds stop compiling or diverge
structural_form: rule:engine-import-confined-to-client-entry checks this at build time
```
