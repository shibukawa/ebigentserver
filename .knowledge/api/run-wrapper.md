---
id: api:run-wrapper
type: api
title: Run Wrapper
---
Entry point call that starts a game or a server from one main function, wrapping the system:ebitengine run call and adding framework options.

```yaml
receives: the game's rules as api:stage-rule-set, declared once beside them so both builds start from the same binding
replaces: the pre engine construction a game entry used to perform, which built concept:session, admitted every concept:agent, and wired transports before anything ran
now_built_later: the session belongs to concept:match-lifecycle, so an entry point starts almost empty
packages:
  - name: engine free half
    holds: framework options, api:roster, the server run call
    imports: no engine, so a tier a server of concept:engine-coupling-tier links nothing renderable
  - name: engine half
    holds: engine options, the game run call, ui:lobby-scene
    wraps: the engine run with options call
    why_split: a single package would drag system:ebitengine into every server and break rule:engine-import-confined-to-client-entry
options:
  framework:
    - max seats, see concept:player-slot
    - whether one screen carries several seats, see concept:view-arrangement and concept:participant-shape
    - accepted input devices among gamepad, mouse, keyboard, consumed by api:input-adapter
  why_devices_are_code: a game cannot accept a device it never wrote an adapter for, so the set is a capability of the build rather than a run value; narrowing it further at run time would be configuration and belongs in data:run-config if it is ever needed
  engine: window size, title, and the rest of the engine option struct, present only in the engine half
not_options:
  - transport choice, which stays a run value under rule:transport-selected-by-capability
  - concept:execution-topology, concept:synchronization-mode, and tick rate, all read from data:run-config
  why: rule:build-tag-only-for-linkage keeps mode selection out of compile time, and a code level option is the same mistake in another form
startup: loads data:run-config, validates data:runtime-resource-budget, then branches on topology
branch:
  dedicated: runs the arbitration hook of api:tick-hooks in a paced loop and never calls the engine run function
  other: runs the engine loop with ui:lobby-scene as the first scene
one_entry_point: both branches live in one main package, so the untagged artifact is playable and hostable, and the dedicated build tag only strips the renderer for deployment
```
