---
id: decision:mirror-popcornweb-dx
type: decision
title: Mirror Popcorn Developer Experience
---
Copy the system:popcornweb developer experience shape rather than inventing a second one.

```yaml
decided: yes
borrowed:
  - an init wizard that produces a compiling project
  - a dev verb that watches, regenerates, rebuilds, restarts, and stops everything together
  - a dev console as one local web UI
  - a project locator file found by walking upward from the working directory
  - typed configuration with generated scaffolds
  - doctor, check, and version verbs
shared_base: system:tinybind is the same dependency under both, so configbind behavior is identical rather than merely similar
deliberate_divergence:
  - generated Go is committed here rather than gitignored, confirmed rather than inherited
  - devbox is not assumed
not_borrowed: routes, queries, storybook, and migrations have no game counterpart
serves: requirement:project-scaffolding, requirement:live-dev-loop
```
