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
failure_mode: an explicitly named file that is missing, unreadable, or a directory fails the load instead of falling back
```
