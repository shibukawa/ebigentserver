---
id: requirement:runtime-config-cleanup
type: requirement
title: Runtime Config Cleanup
---
The run sections keep only what two launches of one artifact may legitimately differ on, and gain the one thing they lack.

```yaml
keep:
  - topology, the concept:execution-topology this process takes
  - listen, the address a host binds
  - transport.enable, the listeners this deployment may open
  - time.mode and time.scale_permille, concept:game-time-control, which concept:training-mode varies per run
  - debug.listen, api:dev-debug-endpoint
add:
  dial_address: >
    a client reaching a dedicated server has no key for its URL. Listen binds,
    and nothing dials — the one run value the file is missing.
  tuning: >
    tick rate, send rate, snapshot interval, and history depth move out of Go
    and into the run sections, per the stage level of concept:configuration-scope.
    They merge with the sync.baseline, sync.speculation_depth, and sync.ack keys
    already declared there, which are the same data:session-tuning-profile
    fields under another name — one of the restatements concept:config-redundancy
    counts.
  tuning_must_agree: >
    every peer of one match must bind the same profile; netplay's client config
    already states it must declare the profile the server runs. This is a
    deployment-wide value, not a per-machine one, and the load should refuse a
    mismatch rather than desynchronize.
remove: the keys requirement:config-file-shape moves to the protocol level or deletes outright
make_it_read: >
  config/runconf is bound by cli alone; neither run nor run/eb imports it, so no
  artifact reads any of this. api:run-wrapper already documents a startup that
  loads data:run-config and branches on topology, and that is the wiring this
  requirement completes.
deployment_file: >
  a run-only file works as a whole configuration for an artifact, since it binds
  only these prefixes — see rule:one-config-file-per-process. It replaces rather
  than layers, which is what makes config.dev.toml viable and a
  difference-only overlay not.
verified_by: a built artifact whose topology, dial address, and tuning come from the file it was pointed at, and which refuses to start when a peer's profile differs
```
