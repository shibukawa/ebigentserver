---
id: api:action-validator
type: api
title: Action Validator
---
Framework hook where game-supplied checks judge an incoming concept:action before it enters the simulation.

```yaml
position: after permission:agent-seat-control accepts the sender, before the simulation applies the action
framework_provides: the hook point, per connection rate limiting, and the escalation ladder
game_provides: the checks themselves, since only the game knows what a plausible move is
two_validator_classes:
  - name: legality
    question: is this action possible under the rules right now
    examples: moving a piece that is not yours to move, building without resources
    runs: inside the simulation path on every simulating peer
    constraint: must be deterministic, so rule:codegen-rejects-nondeterministic-types applies; a legality disagreement between peers is a desync
  - name: plausibility
    question: could an honest client have produced this
    examples: a position jump exceeding maximum speed, action rate beyond humanly possible, inputs arriving for future ticks
    runs: only on the authoritative side, outside the simulation
    constraint: free to use heuristics and floats, because rejecting here never touches simulation state
escalation: drop the action, flag the connection, then disconnect on repetition; thresholds in data:session-tuning-profile
relation_to_authority: under term:server-authority this is the cheat boundary; under term:rollback legality checks run everywhere identically and plausibility applies only where an authoritative host exists
recorded: rejections are events worth keeping, since a cluster of them is either a cheat or a client bug
```
