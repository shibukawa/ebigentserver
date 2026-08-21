---
id: concept:view-arrangement
type: concept
title: View Arrangement
---
Whether every seat sees the same view or each holds its own. This is the camera, and it is independent of whether the seats share a process.

```yaml
arrangements:
  - name: shared
    camera: one view of the whole world, or a fixed frame every seat reads
    examples: fighting games, board and puzzle games, most couch party games
  - name: per_agent
    camera: one view per seat, following that seat
    examples: shooters, anything with a per player position
split_screen_is_both: >
  several seats on one machine, each with its own camera. It is why the
  camera is not the question a project is asked: shared against per_agent
  cannot express it, since it is per_agent cameras on a shared machine.
  What flow:project-init asks instead is whether a machine may seat more
  than one person, and the camera inside that is a rendering choice made
  in the draw call — one frame or several viewports.
orthogonal_to: concept:execution-topology, which is where the seats sit
four_real_quadrants:
  - shared camera, one process: couch versus play; no link, so no concept:synchronization-mode
  - shared camera, separate processes: online fighting games — a link exists, so synchronization does too, and term:rollback is the usual choice for the genre
  - per_agent camera, one process: split screen on one console; no link, so no synchronization
  - per_agent camera, separate processes: online shooters, usually term:server-authority with concept:client-prediction
sync_follows_the_process_boundary_not_the_camera: >
  a link is what has to be kept consistent. Sharing a camera across two
  machines does not remove the link, it only means both machines render the
  same thing — which is precisely the case rollback was invented for, since
  a shared camera shows a mispredicted frame to both players at once.
camera_suits_a_sync_mode:
  shared: term:rollback fits, because the whole world is on screen and a
    correction is visible immediately; it demands
    rule:deterministic-simulation-required-for-rollback
  per_agent: term:server-authority fits, because a seat only sees its own
    surroundings and a correction outside them costs nothing
visibility_constraint:
  first_half: >
    a shared camera forces the global scope of concept:visibility-scope. A
    seat cannot be shown what it was not sent, so if the frame contains the
    whole world, the observation must too. A game with term:fog-of-war that
    matters cannot have a shared camera, whatever its topology.
  second_half: >
    one process makes the scope unenforceable between the people present.
    The guarantee is that hidden data is never sent; in one process it was
    never sent anywhere, and on one screen it is in front of everyone. A
    split camera there is a convention, not a boundary — screen watching is
    the well known consequence.
  consequence: only per_agent cameras in separate processes make
    policy:observation-scoped-information a real boundary
  unchanged: rule:observation-content-owned-by-game still builds one
    concept:observation per seat in every quadrant; what changes is whether
    the arrangement can keep the promise
not_an_axis: what controls a seat — actor:human-agent, a bot, or actor:remote-agent — is orthogonal again, since decision:agent-as-central-abstraction makes them indistinguishable to the session
remote_controller_case:
  shape: shared camera, one process, seats fed from elsewhere — a phone as a gamepad, the screen on a television
  transport_carries: concept:action upstream only, with no concept:state-synchronization, because there is one view and it is already local
```
