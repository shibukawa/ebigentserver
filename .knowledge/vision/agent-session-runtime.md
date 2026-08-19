---
id: vision:agent-session-runtime
type: vision
title: Agent Session Runtime
---
Build a runtime that lets any agent join a game session, not a game-specific dedicated server.

```yaml
core_claim: human, bot, and replay players are the same concept:agent; they differ only in how concept:observation becomes concept:action
topology_claim: standalone, listen server, dedicated server, and p2p are placements of concept:agent and concept:session, expressed by concept:execution-topology
language: go
first_target: system:ebitengine
enables:
  - multiplayer play
  - human plus AI cooperative play
  - automated playtest via flow:automated-playtest
  - game AI growth via flow:behavior-profile-derivation
  - replay and offline simulation via concept:simulation-mode
  - bounded production operation via requirement:production-runtime-safety
grounded_by:
  - decision:agent-as-central-abstraction
  - decision:separate-topology-from-synchronization
```
