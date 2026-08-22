---
id: rule:one-config-file-per-process
type: rule
title: One Config File Per Process
---
Each process reads exactly one configuration file.

```yaml
mechanism: configbind selects the first readable candidate and never merges files
consequence: layering across two TOML files is unavailable, which is why decision:one-config-file-many-sections puts every setting in one file split by prefix
layering_substitute: environment variables and CLI options carry per-deployment differences a second file would otherwise carry
local_override: listing a project-local candidate ahead of the user and system config directories lets a developer replace the shared file rather than extend it
a_deployment_file_still_works: >
  a built artifact binds only the run prefixes, so a file holding nothing but
  those sections is a complete configuration for it — passing
  --config-path config.dev.toml replaces ebigent.toml and loses nothing the
  artifact wanted. This works because the game scope of
  concept:configuration-scope is generated rather than bound: were it read
  from the file, replacing the file would drop it.
  What stays unavailable is the difference-only file — ebigent.toml supplying
  run defaults with a second file overriding two keys. That is what
  layering_substitute is for.
failure_mode: an explicitly named file that is missing, unreadable, or a directory fails the load instead of falling back
```
