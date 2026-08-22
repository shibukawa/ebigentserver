---
id: concept:engine-coupling-tier
type: concept
title: Engine Coupling Tier
---
How far a game's simulation is separated from system:ebitengine, which decides what a server process may link and what guarantees it keeps.

```yaml
why: a typical Ebitengine game keeps logic inside the Game struct and swaps scenes by swapping Game implementations, so requiring an engine-free rules package first is an adoption barrier
tiers:
  - tier: a
    game_shape: simulation lives in packages that never import the engine
    server_form: pure go artifact under the dedicated build tag, see rule:build-tag-only-for-linkage
    keeps: term:determinism, bit identical replay of data:episode-log, concept:behavior-distillation, static container image
    enforced_by: rule:engine-import-confined-to-client-entry
  - tier: b
    game_shape: Game struct is coupled, but the arbitration step is separable as its own method
    server_form: engine linked, RunGame never called, api:run-wrapper drives only the arbitration hook of api:tick-hooks
    requires: a display connection at import time, see decision:xvfb-for-coupled-game-servers
    loses: cross architecture digest equality, since floats and wall clock may remain in game code
    intended_as: the mainstream adoption path
  - tier: c
    game_shape: arbitration cannot be separated from rendering
    server_form: full RunGame under a virtual display with software rendering
    cost: heaviest runtime, useless draw work every frame
migration: tier b becomes tier a by moving the arbitration hook into a package that does not import the engine; no api:run-wrapper change is involved
same_api_across_tiers: the wrapper, api:roster, and api:tick-hooks are identical, so a game changes tier without rewriting its seams
```
