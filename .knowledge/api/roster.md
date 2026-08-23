---
id: api:roster
type: api
title: Roster
---
Mutable list of who will play, filled before a concept:session exists and frozen into one when play starts.

```yaml
holds: one entry per prospective concept:player-slot, carrying seat id, controller kind, and identity
seat_state: empty, human, or bot — the three a roster distinguishes
local_and_remote_are_results: >
  whether a human or a bot sits locally or across a link follows from
  concept:execution-topology and from which process took the host part, so it
  is reported rather than declared. A declaration says bot; where the bot runs
  is not its business.
where_a_bot_runs: the process holding the host part, or the dedicated server when there is one
seat_composition_is_declared_at_build: >
  how many seats, how they divide into teams, whether a seat admits a human, a
  bot, or either, and whether an empty seat is refused or filled with a bot —
  all of it is the game's contract and fixed at build in the protocol level of
  concept:configuration-scope. A roster fills the seats it was given; it never
  invents one.
local_operations:
  - join a local seat from a device press, bounded by the accepted devices of api:run-wrapper
  - add a bot seat
  - leave
  - mark ready
remote_operations:
  - admit a remote seat, driven by flow:session-admission rather than by any screen
  - report departure before play, distinct from concept:agent-departure-policy which governs departure during play
sight: a change callback, which ui:lobby-scene renders and a custom scene may use instead
seeding: the slot table of data:run-config pre fills entries, so a run may skip gathering entirely and start headless
finalize: builds concept:session, admits every entry through api:agent-interface, and wires transports, producing the running match of concept:match-lifecycle
raw_api_is_the_contract: ui:lobby-scene is only a default caller, so a game may replace the screen without losing admission, discovery, or bot seating
identity: seat identity comes from data:session-ticket where one exists; rule:identity-token-not-accepted-by-session still holds
```
