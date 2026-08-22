---
id: api:tick-hooks
type: api
title: Tick Hooks
---
The three game supplied steps a term:tick passes through, and the only places a game meets the framework during play.

```yaml
hooks:
  - name: intake
    does: read devices through api:input-adapter and submit one data:player-input for each local seat
    runs_on: client and listen builds only
    rule: rule:no-engine-input-in-game-logic still applies, so the hook emits concept:action rather than device state
  - name: arbitrate
    does: commit the inputs of every concept:player-slot into concept:world-state for this tick
    runs_on: whichever process holds term:server-authority
    is: the only hook a concept:dedicated-server-mode process runs
    constrained_by: rule:deterministic-tick-commit, so ordering is by slot id and never by arrival
    policy_lives_elsewhere: input recency and queueing in data:session-tuning-profile intake mode, rejection in api:action-validator
    not_free_form: a hook that merged by wall clock or arrival order would break term:determinism and rule:deterministic-simulation-required-for-rollback
  - name: apply
    does: take the delivered concept:agent-view or data:state-delta and update what the client draws
    runs_on: client and listen builds only
driving:
  server_authoritative: arbitrate runs on the session clock, independent of engine frame rate, so a dedicated build and a replay tick identically
  rollback: arbitrate runs inside apply, replaying from the corrected tick as late input arrives, see term:rollback
  chosen_by: concept:synchronization-mode, not by the game, per decision:input-timing-owned-by-sync-mode
tier_b_note: in tier b of concept:engine-coupling-tier the arbitrate hook may still contain engine calls; input reads inside it return zero values on a server, which is harmless
```
