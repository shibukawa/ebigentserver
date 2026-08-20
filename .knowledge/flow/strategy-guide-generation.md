---
id: flow:strategy-guide-generation
type: flow
title: Strategy Guide Generation
---
Render approved chips and benchmark results as a human-facing strategy guide, since a chip already is advice with evidence.

```yaml
flow:
  trigger: a game's data:behavior-chip library and ui:chip-benchmark results exist
  steps:
    - id: select
      action: pick chips by tag and by measured win-rate contribution, strongest first
    - id: narrate
      actor: actor:llm-agent
      action: render each chip's condition and action as prose; the data:derived-predicate names are already the guide's vocabulary
    - id: evidence
      action: attach concrete situations from concept:behavior-evidence as worked examples, replayable via actor:replay-agent
    - id: caveat
      action: state counterexamples and matchup dependence from ui:chip-benchmark ablation, so the guide does not overclaim
example: tic-tac-toe never-lose guide from the expert loadout's chips; fighting game matchup pages per opponent tactic tag
why_cheap: no new analysis runs; the pipeline's review artifacts are the guide's sources, only the rendering is new
```
