---
id: concept:agent-departure-policy
type: concept
title: Agent Departure Policy
---
Game-owned choice of what a concept:session does with a seat when its agent is declared lost.

```yaml
detection: silence deadline of api:sequence-ack-layer, or transport close
options:
  - name: abort_session
    fits: one to one competitive play where the match is meaningless without both sides
  - name: continue_without
    fits: team play; the seat empties and the entity is removed
  - name: designated_proxy
    fits: any session where a player may step away deliberately
    mechanism: concept:agent-proxy-designation, chosen by the player in advance
  - name: ai_takeover
    fits: team play where an empty seat ruins the match for everyone else
    mechanism: seat a actor:behavior-tree-agent in the same slot
    why_free: human and bot are the same concept:agent, so no substitution path is needed, see decision:agent-as-central-abstraction
    refinement: instantiate it from the departed player concept:behavior-profile when one exists
  - name: persist_entity
    fits: persistent worlds; the entity stays in concept:world-state and the player rejoins later
    rejoin: normal admission, see decision:no-mid-session-reconnect
selection: per game, and may differ by session kind rather than being a framework constant
seat_reuse: a returning player may or may not get the same seat; that is part of this policy
```
