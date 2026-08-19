---
id: system:fixpointcs
type: system
title: FixPointCS
---
External multi-precision fixed-point library whose operations are bit-identical across languages and compilers.

```yaml
url: https://github.com/XMunkki/FixPointCS
provides: Fixed32 and Fixed64 primitives, sqrt, rcp, exp, log, sin, cos, atan2 with stated precision
design_goal: bit identical results across languages and compilers, the property term:determinism needs
languages: c sharp, java, c plus plus
go_port: none published; framework must supply one
usage_note: upstream is a low level building block, not an application facing api
license: verify before vendoring
role: recommended source for api:fixed-point-math core operations
```
