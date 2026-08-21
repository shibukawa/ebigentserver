---
id: requirement:layered-configuration
type: requirement
title: Layered Configuration
---
Every configurable value resolves through one declared struct and one fixed override order.

```yaml
sources: TOML file, environment variable, CLI option
order: rule:config-precedence-fixed
mechanism: system:tinybind configbind, see decision:configbind-for-all-config
file: one ebigent.toml, sectioned into data:build-config for the toolchain and data:run-config for a built artifact, see decision:one-config-file-many-sections
game_extension: a game declares its own config struct and receives TOML keys, env names, and CLI options without writing a parser
scaffold: the config verb of api:game-cli renders declared fields as commented TOML or dotenv
audit: startup logs each effective value with the layer that set it, secrets masked
```
