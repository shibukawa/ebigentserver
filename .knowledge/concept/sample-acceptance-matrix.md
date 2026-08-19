---
id: concept:sample-acceptance-matrix
type: concept
title: Sample Acceptance Matrix
---
Executable mapping from each concept:coverage-matrix axis to a fixed sample configuration and assertion.

```yaml
entry_fields:
  - axis and value
  - sample id
  - topology, synchronization, timing, transport, tuning profile, and controller pairing
  - deterministic setup and assertion
  - test level: unit, process, browser, network impairment, or load
configurations:
  ttt_step_local: sample:tic-tac-toe, standalone, command, step, local transport
  reversi_step_local: sample:reversi, standalone, command, step, local transport
  pong_realtime_network: sample:pong, dedicated, world state, realtime, system:webtransport
  tron_realtime_loss: sample:tron, dedicated, world state, realtime, system:webtransport with network impairment
  maze_realtime_team: sample:cooperative-maze, dedicated, world state, realtime, system:webtransport, human plus ai team
  dm_slow_hybrid: sample:dungeon-master, dedicated, hybrid, slowed realtime, system:webtransport, human and ai role variants
  rts_realtime_hybrid: sample:rts-lite, dedicated, hybrid, realtime, system:webtransport with network impairment
coverage:
  controller_pairing:
    human_vs_human: ttt_step_local, admission and legal turn assertion
    human_vs_ai: reversi_step_local, shared action contract assertion
    ai_vs_ai: reversi_step_local, deterministic terminal result assertion
  relationship:
    competitive: reversi_step_local
    cooperative: maze_realtime_team
    team_based: maze_realtime_team
    mixed_loyalty: uncovered
  symmetry:
    symmetric: ttt_step_local
    asymmetric: dm_slow_hybrid
  timing:
    turn_based: ttt_step_local
    real_time: pong_realtime_network
  synchronization:
    command: reversi_step_local
    world_state: pong_realtime_network
    hybrid: rts_realtime_hybrid
  visibility:
    full_world: ttt_step_local
    partial: rts_realtime_hybrid
    role_specific: dm_slow_hybrid
    team_only: maze_realtime_team
  composition:
    human_ai_mixed_team: maze_realtime_team
  observers:
    spectator: tron_realtime_loss
  continuity:
    player_replacement: tron_realtime_loss
coverage_rule:
  - optional variants do not count
  - an axis value counts only after its fixed configuration runs in automation
  - failure blocks the framework claim supported by that axis
required_scenarios:
  - protocol mismatch and invalid admission
  - packet loss, reordering, slow receiver, and disconnect
  - replay data:state-checkpoint equality
  - data:runtime-resource-budget boundary
known_gap: mixed loyalty remains uncovered until a backlog sample is promoted
```
