---
id: decision:configbind-for-all-config
type: decision
title: Configbind For All Config
---
Use system:tinybind configbind for both configuration files and add no framework-specific layer.

```yaml
decided: yes
gained: TOML plus env plus CLI merge, generated scaffolds, ${NAME} expansion inside file values, secret masking, and per-key provenance
not_built: no custom parser, no configurable merge order, no dotenv reader — configbind reads the process environment only, so a scaffolded .env needs a shell or dotenv loader in front
constraint: the supported TOML is a configuration-focused subset without inline tables or nested arrays, so repeated settings are arrays of tables
enum_is_not_enforced: the enum tag reaches neither the generated code nor the loader in tinybind-go v0.5.17; generation consults it only when checking a dependon condition's values. An unlisted value binds silently, so every allowlist is restated in a Validate method and the load runs those before returning
typo_risk: an unknown TOML key applies silently while an unknown CLI option fails loudly; startup compares overlay keys against the declared set and rejects strays, scoped to the prefixes this process owns per decision:one-config-file-many-sections
serves: requirement:layered-configuration
```
