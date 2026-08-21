---
id: data:build-config
type: data
title: Build Config
---
Toolchain sections of `ebigent.toml`, and the file that marks a project root.

```yaml
file: ebigent.toml, shared with data:run-config under decision:one-config-file-many-sections
locator: every verb walks upward from the working directory until it finds this file, the way system:popcornweb finds popcornweb.toml
sections:
  - project: module path and pinned go toolchain, the latter checked against the host by the doctor verb
  - targets: one entry per concept:build-target with its cmd entry point, goos, goarch, and its build tags; two entries may share one entry point and differ only in tags, which is how the listen and headless forms of one server are declared under rule:build-tag-only-for-linkage
  - dev: default target to run, watch roots, ignore globs, debounce, ui:dev-console address
  - behavior: chip library path, corpus root, analysis skill directory
binding: one configbind.Bind per section, so every key also carries an env name and a CLI option under rule:config-precedence-fixed
ignored_by: the built game artifact, which registers none of these prefixes and so never reads them
```
