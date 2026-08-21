---
id: requirement:control-plane-decoupling
type: requirement
title: Control Plane Decoupling
---
Framework must run without any specific control plane implementation.

```yaml
contract: signed data:session-ticket only, see flow:session-admission
not_required: system:popcornweb
out_of_scope: concept:control-plane features, see decision:control-plane-features-out-of-scope
decision: decision:independent-from-popcornweb
```
