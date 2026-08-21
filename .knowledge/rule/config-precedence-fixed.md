---
id: rule:config-precedence-fixed
type: rule
title: Config Precedence Is Fixed
---
Configuration sources merge in one order that no project may change.

```yaml
order: default < TOML file < environment variable < CLI option
enforced_by: system:tinybind configbind; the order is not a setting
env_name: derived from the CLI long option, hyphens and dots becoming underscores, then uppercased
file_selection: the first readable candidate only — an explicit path option, then project-local paths, then user and system config directories; files are never merged
applies_to: data:build-config and data:run-config alike
distinct_axis: the game, run, and session layering in data:run-config is about who chooses a value, not which source file it came from; a data:session-ticket overriding a run value is admission, not configuration
```
