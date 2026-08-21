---
id: decision:reuse-tinybind-codegen
type: decision
title: Reuse Tinybind Code Generation
---
Extend system:tinybind with CBOR generation instead of writing new codegen.

```yaml
decided: yes
reused: struct analysis, json binding, config binding, cli binding
added: cbor generation for both encoding profiles
shared_with: system:popcornweb, which depends on the same codegen base
```
