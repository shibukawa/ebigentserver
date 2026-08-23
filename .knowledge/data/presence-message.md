---
id: data:presence-message
type: data
title: Presence Message
---
Ambient co-presence data such as cursors, pointers, and emotes that conveys that someone else is there.

```yaml
examples: cursor position, camera or look direction, selection highlight, ping marker, emote, typing indicator
property: conveys presence, never affects game progression
not_part_of: concept:world-state, so it appears in no data:snapshot and no data:state-delta
authority: client asserted, never validated, so it can never justify an authoritative decision
encoding: concept:cbor-wire-profile, small fixed layout
rate: independent of tick rate, usually lower; a presence_rate field of data:session-tuning-profile
delivery: rule:presence-superseded-not-retransmitted
analysis: rule:presence-excluded-from-simulation-and-analysis
visibility: still filtered through concept:agent-view; see the leak note in policy:sight-scoped-information
```
