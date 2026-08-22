---
id: data:configuration-surface
type: data
title: Configuration Surface
---
Every declared setting in the framework, counted by site, tier, and whether anything reads it.

```yaml
totals:
  go_option_fields: 95, spread over eleven structs a game author fills in by hand
  toml_leaf_keys: 36, of which 16 are data:build-config and 20 are data:run-config
  cli_verb_options: 20, across init, build, config, analyze, and merge
  note: counts are declaration sites, not what one game sets; the step2-lobby tutorial fills 43 of the 95 and writes no TOML at all
go_sites:
  - run.Options: 4 — name, devices, shared_screen, max_local_seats; build facts a game states once
  - run.Binding: 5 — slots, config factory, agent factory, protocol_version, evaluation_version
  - run.ServeOptions: 5 — headless matches, seed, time, record, on_match
  - run.RecordOptions: 6 — data:episode-log destination and header
  - eb.Options: 13 — the widest single struct; framework declaration, rules, scene, lobby, network, window
  - eb.LobbyOptions: 4 — prompt, auto_start, no_bots, background
  - lan.Options: 12 — api:lan-preset wire declaration; five restate what run.Binding already holds
  - session.Config: 16 — one match, built per match by a game supplied factory
  - session.TuningProfile: 12 — data:session-tuning-profile, deliberately defaultless per decision:no-framework-tuning-defaults
  - statesync.Codec: 7 — every field mechanical, six already emitted by the cbor generator
  - budget.Budget: 11 — data:runtime-resource-budget; lan supplies an internal default, nothing else does
toml_sites:
  - build tier: project 2, build.target 7 per entry, dev 5, behavior 2
  - run tier: run 3 scalar, transport 4, sync 4, time 2, slot 3 per entry, episode 3, debug 1
unread_run_tier: >
  config/runconf is bound only by cli; neither run nor run/eb imports it, so
  all 20 run keys reach no artifact. api:run-wrapper documents a startup that
  loads data:run-config and branches on topology, and no code does that yet.
keys_with_no_implementation:
  - run.sync.mode: delay, rollback, server_authoritative, hybrid — concept:synchronization-mode has no Go representation at all
  - run.time.scale_permille: session.TimeControl offers only Paced and Unlimited, so scaled and step cannot be expressed
  - run.episode.sample_percent: run.RecordOptions has no sampling field
  - run.debug.listen: api:dev-debug-endpoint has no consumer
  - run.transport.enable: api:lan-preset hardcodes system:websocket
enum_drift: run.time.mode declares four values against session.TimeControl's two, so the file promises more than the type can hold
see_also: concept:config-redundancy for what restates what, rule:config-tier-placement for where a new setting goes
```
