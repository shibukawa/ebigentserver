---
id: requirement:match-stage-vocabulary
type: requirement
title: Match And Stage Vocabulary
---
The code adopts the four names of concept:configuration-scope, so one thing has one name across catalog, docs, and API.

```yaml
renames:
  simulation_to_stage_rule_set: >
    done. session.Simulation is session.StageRuleSet, TickSimulation is
    TickStageRuleSet, Config.Simulation is Config.RuleSet, and the type a game
    declares is RuleSet. api:stage-rule-set carries the concept.
  scene_to_screen: >
    done in code. eb.Scene is eb.Screen, since stage is the framework word for
    a rule set and scene is what the wider industry calls the same thing.
    ui:lobby-scene names a screen and keeps its id.
  seat_kinds: >
    done. SeatKind is Empty, Human, Bot; Seat.Local reports where the occupant
    runs, and Seat.LocalHuman and Seat.LocalBot read the pair. Roster.Take
    takes the local flag, which the named join helpers supply.
new_boundary:
  stage_runner: >
    Match, Session, the eb app, Networking and the lan preset are each
    parameterized on one S/A/O triple fixed at the entry point, which is why a
    second rule set means a second match. Erasing the triple behind a
    non-generic runner at the match boundary is what makes
    decision:codec-set-per-stage reachable; Hosting.Rebind already proves the
    link outlives the session.
  stage_index: one byte in the statesync packet header, selecting the codec and marking a packet from a stage already left as stale
declaration_site: >
  NewStage takes the stage rule set and its schema as type arguments, and
  requirement:config-codegen generates the codec from that call — the same
  call-site-is-the-ask shape requirement:cborbind-migration adopts upstream.
verified_by: a sample running two stages of different state types over one gathered roster, with no re-gathering between them
```
