---
id: decision:independent-from-popcorn-wave
type: decision
title: Framework Independent From Popcorn Wave
---
Game framework does not depend on system:popcorn-wave; both depend only on system:tinybind.

```yaml
decided: yes
shared_dependency: system:tinybind
integration_point: data:session-ticket only
consequence: any concept:control-plane implementation can front the framework
serves: requirement:control-plane-decoupling
```
