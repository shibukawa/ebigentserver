---
id: requirement:config-file-shape
type: requirement
title: Config File Shape
---
What flow:project-init writes must match the four levels of concept:configuration-scope: nothing declared that is derivable, nothing missing that a build needs.

```yaml
add_protocol_section:
  status: done — [protocol] holds package, title, shape, realtime, view, devices, sync, and the seat composition
  why: the protocol level of concept:configuration-scope had no home in the file
  keys:
    - package path with its subpath, per decision:module-path-is-game-identity
    - participant shape, concept:participant-shape
    - realtime intensity, concept:realtime-intensity
    - view arrangement, concept:view-arrangement
    - accepted input devices
    - seat composition: count, team division, which seats admit human or bot or either, and whether an empty seat is refused or filled
  source: every one is already asked by flow:project-init or already fixed by the answers
move_into_protocol:
  - sync.mode, today under run; a synchronization mode is settled at build and rule:build-tag-only-for-linkage's reasoning applies to a run key just as well
  - transport.require_unreliable and its siblings, today under run; they follow from concept:realtime-intensity rather than from a launch
delete:
  - run.slot: the match level takes no configuration file; api:roster fills what protocol declared
  - run.episode: recording destination is internal and settled at build
  - run.evaluation_version: derived from the rule set that produced the signal
  - behavior.corpus: dropped from what the scaffold writes; the key stays
    bound with its default, since analyze and doctor read it and it is a
    build-tier path rather than the run-tier destination run.episode was
review:
  build_target_array: >
    flow:project-init already derives the entry point set from the shape and
    says so rather than asking. An explicit array of tables then restates what
    the shape decided. Keep it only for what the shape cannot say — goos,
    goarch, and tags for a target beyond the generated set — and derive the
    rest.
  behavior_library: unresolved; the chip library path is not the corpus and may still need declaring
verified_by: a scaffolded project whose file contains no key derivable from another, and whose build needs no key the file lacks
```
