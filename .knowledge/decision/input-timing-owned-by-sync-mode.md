---
id: decision:input-timing-owned-by-sync-mode
type: decision
title: Input Timing Owned By Synchronization Mode
---
Which tick a client stamps on data:player-input, and what happens to late input, is defined per concept:synchronization-mode, not globally.

```yaml
decided: yes
per_mode:
  - mode: term:rollback
    stamp: the local simulation tick; remote peers rewind to it on arrival
  - mode: term:delay-buffering
    stamp: local tick plus the fixed delay window
  - mode: term:server-authority
    stamp: the client estimate of the server tick at arrival time; the session decides whether late input is dropped or applied next tick
  - mode: agent driven step of decision:dual-mode-agent-pacing
    stamp: assigned by the session, since the agent advances the clock
consequence: clock estimation machinery is required only by server authoritative realtime play, so it lives with that mode instead of in the core
late_input_policy: a per mode setting in data:session-tuning-profile
```
