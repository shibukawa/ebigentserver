---
id: decision:simulation-not-game
type: decision
title: The Game Side Contract Is Not Named Game
---
api:stage-rule-set carries any name but Game, because a client entry point already links one.

```yaml
problem: >
  system:ebitengine declares its own Game interface, Update / Draw /
  Layout. A client entry links both it and the framework contract, so two
  unrelated interfaces named Game meet in one file, and neither name says
  which one advances the world.
choice: StageRuleSet for the contract and RuleSet for the type a game declares to implement it, per the stage level of concept:configuration-scope
also_renamed:
  - TickGame becomes TickStageRuleSet, the realtime extension adding advance
  - the session Config field becomes RuleSet
earlier_name: >
  the contract was called Simulation before the four levels of
  concept:configuration-scope were settled. Stage rule set says the same
  thing and says which level owns it.
displaced_meaning: >
  simulation already named the headless run mode, which is now
  concept:training-mode and concept:training-farm. That word moved because
  its purpose is producing data:episode-log for concept:behavior-distillation,
  which training names better than simulation did.
kept: >
  simulation in its plain sense stays everywhere it means the world
  advancing — rule:no-float-in-simulation,
  rule:deterministic-simulation-required-for-rollback, the simulate verb of
  api:game-cli, and the simulation concept:build-target. Those now agree
  with the interface name instead of competing with it.
consequence: rule:engine-import-confined-to-client-entry keeps the two apart structurally; this decision keeps them apart by name as well
```
