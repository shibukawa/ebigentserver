---
id: concept:match-lifecycle
type: concept
title: Match Lifecycle
---
The loop a process repeats around concept:session, so a session lasts one match rather than the whole process.

```yaml
why: a real game gathers its players after launch, so a roster cannot be known at startup and the session cannot be built there
states:
  - idle: nothing gathered yet
  - gathering: api:roster fills from local presses, api:lan-discovery, api:manual-signaling-token, and flow:session-admission
  - running: the session exists and concept:session-lifecycle governs it
  - ended: outcomes reported, then back to idle
relation_to_session_lifecycle: this loop wraps it; created through ended of concept:session-lifecycle all happen inside the running state here
difference_from_continuous_match_loop: concept:continuous-match-loop is unattended corpus generation that picks its own pairings; this is one serving process waiting for whoever shows up
per_mode:
  client: gathering is a screen, see ui:lobby-scene
  listen: gathering both shows a screen and accepts remote seats
  dedicated: gathering is a listener alone, and returning to idle is how one server process serves match after match
  standalone: gathering may be skipped when data:run-config already names every slot
unchanged_by_this: term:server-authority placement, concept:synchronization-mode, and every rule about ticks, which all begin at running
```
