---
id: api:fixed-point-math
type: api
title: Fixed Point Math
---
Deterministic scaled-integer math used by the simulation core in place of the Go math package.

```yaml
operations: add, sub, mul, div, sqrt, rcp, exp, log, sin, cos, atan2
angles: binary angle measurement, full turn spans the integer range, so wrapping is free and pi is never stored
recommended_core: port of system:fixpointcs, recommended rather than mandated
substitution: a game may supply its own core if it preserves term:determinism
excluded: Go math package, see rule:no-float-in-simulation
serves: decision:fixed-point-numeric-representation
```
