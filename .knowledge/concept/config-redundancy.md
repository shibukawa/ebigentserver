---
id: concept:config-redundancy
type: concept
title: Config Redundancy
---
Settings a game states more than once, and the derivation that would let it state each one only once.

```yaml
measured_on: the step2-lobby tutorial, 43 assignments for tic-tac-toe, 6 of them restatements
restated_in_go:
  - name: run.Options.Name and lan.Options.Name, the literal "tictactoe" typed twice in two files
  - protocol: run.Binding.ProtocolVersion and lan.Options.Protocol, both data:game-version
  - tuning: session.Config.Tuning and lan.Options.Tuning, both the same data:session-tuning-profile call
  - projection: session.Config.Simulation already carries Project; lan.Options.Project names the method again
  - slots: run.Binding.Slots and session.Config.Slots, both the game's concept:player-slot set
  - seed: eb.Options.Seed threads through the Config factory into session.Config.Seed
why_lan_cannot_take_binding: >
  lan.Options is parameterized on four types and run.Binding on three. The
  extra one is the delta type, and it appears in exactly one field —
  statesync.Codec. Boxing the codec behind an interface erases it, after
  which the preset can read name, protocol, tuning, and projection off the
  binding instead of asking for them again.
generable_rather_than_written:
  - statesync.Codec: six of seven fields are already emitted by the cbor generator as AppendCBORTo, DecodeCBORFrom, Diff, Apply; only Clone is hand-written, and it is hand-written because a value copy aliases a slice, which is a defect the generator would not produce
  - run.Binding.ProtocolVersion: already sourced from the generated constant, so nothing has to be typed
restated_across_tiers: >
  data:run-config declares sync.baseline, sync.speculation_depth, sync.ack,
  episode.dir, episode.mode, time.mode, and evaluation_version, each of which
  already exists as a Go field. Because config/runconf reaches no artifact,
  the pair is not yet a conflict; wiring it without deciding precedence would
  make it one.
default_is_allowed_here: >
  decision:no-framework-tuning-defaults forbids a default tick rate, bandwidth
  budget, or lag policy, because those differ per genre. It says nothing about
  plumbing — window size, browse window, budget, lobby background, record mode
  — where a default is the ordinary way to keep an entry point short.
```
