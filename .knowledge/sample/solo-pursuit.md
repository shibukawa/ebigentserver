---
id: sample:solo-pursuit
type: sample
title: Solo Pursuit
---
One person, two enemies, and no network: the smallest arrangement in which the whole distillation loop closes.

```yaml
players: 1 human, 2 non player pursuers
why_a_solo_game_is_on_a_session_framework: >
  the enemies. Each holds a concept:player-slot and decides through
  api:agent-interface, so every enemy decision reaches data:episode-log
  with the sight it was made from. A hand written enemy update
  function produces no such record, and that record is the only input
  flow:behavior-tree-synthesis has.
hooks_that_run_for_a_non_player: the intake and apply steps of api:tick-hooks; only arbitration is central, and in a solo game it is central in the same process
enemy_kinds_are_seat_labels: the kind travels in the episode header, so a corpus separates per kind — distilling three mixed pursuers as one would produce a policy none of them had
timing: realtime tick loop, concept:standalone-mode, no transport of any kind
entries: a window build, a headless build that fills every seat with an agent, and a distillation command
loop_it_closes:
  - play, attended or not, recording data:episode-log
  - mine each kind's decisions into data:behavior-chip over a shared vocabulary
  - compile the approved chips to Go per decision:behavior-tree-compiled-to-go
  - seat the compiled enemy and play again
proven:
  equivalence: a match with the distilled enemies is identical to one with the hand written ones, tick for tick and checkpoint for checkpoint
  vocabulary_is_not_the_policy: both kinds are mined with the same predicates over the same corpus and produce different decision lists
  corpus_balance: the sample is tuned so a run holds both escapes and captures, since a corpus of one outcome teaches nothing
exercises:
  - api:run-wrapper and api:roster with a lobby that seats one person and fills the rest
  - concept:match-lifecycle returning to gathering after each match
  - data:state-checkpoint as the equivalence test rather than as documentation
deliberately_absent: transports, visibility scoping, and any bot cleverness — the enemies are minimal because the depth is meant to come from distillation
relation_to_the_ladder: not a rung of concept:sample-progression; it is what flow:project-init generates for the solo shape of concept:participant-shape, carried far enough to prove the loop
```
