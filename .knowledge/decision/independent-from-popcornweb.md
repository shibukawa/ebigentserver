---
id: decision:independent-from-popcornweb
type: decision
title: Framework Independent From Popcorn Web
---
Game framework does not depend on system:popcornweb; both depend only on system:tinybind.

```yaml
decided: yes
shared_dependency: system:tinybind
integration_point: data:session-ticket only
consequence: any concept:control-plane implementation can front the framework
serves: requirement:control-plane-decoupling
```
