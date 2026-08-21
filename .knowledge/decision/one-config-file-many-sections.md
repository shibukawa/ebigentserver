---
id: decision:one-config-file-many-sections
type: decision
title: One Config File Many Sections
---
One `ebigent.toml` holds every setting; a section prefix selects which struct binds it.

```yaml
decided: yes
file: ebigent.toml, described by data:build-config
readers:
  - the ebigent tool binds the toolchain sections
  - the built artifact binds the data:run-config sections and the game's own
mechanism: one configbind.Bind per prefix; a process registers only the prefixes it owns, and a key belonging to no registered prefix is ignored
why_one_file: rule:one-config-file-per-process means a process reads a single TOML, so a second file would only be reachable by whichever process named it — a shared file lets both read the same source of truth
why_sections_not_one_struct: a build choice is fixed at link time by concept:build-target while a run choice is picked per launch; separate structs keep a runtime option from claiming to change something already compiled out
consequence: the stray-key check of decision:configbind-for-all-config must be scoped to the prefixes this process owns, since every process legitimately sees sections belonging to the other
bridge: the dev verb of api:game-cli reads the toolchain sections and passes run values to the child as CLI options
serves: requirement:layered-configuration
```
