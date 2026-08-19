---
id: rule:deterministic-tick-commit
type: rule
title: Deterministic Tick Commit
---
Every simulation tick commits actions, events, and parallel results in a stable order independent of arrival and scheduling.

```yaml
input_order:
  - snapshot eligible slots at tick start
  - order slots by stable slot id
  - order multiple actions from one slot by per slot action sequence
  - resolve late or duplicate actions before simulation using concept:synchronization-mode
commit:
  - simulation has one logical commit point per term:tick
  - parallel work may compute privately but merges by stable entity key
  - emitted events use commit order plus per emitter sequence
forbidden: transport arrival order, goroutine scheduling, pointer identity, unsorted container traversal
verification: emit data:state-checkpoint at configured checkpoints
serves: term:determinism, actor:replay-agent, term:rollback
```
