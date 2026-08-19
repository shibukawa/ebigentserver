---
id: concept:coverage-matrix
type: concept
title: Coverage Matrix
---
The independent design axes the sample suite must span, used to detect untested framework claims.

```yaml
axes:
  - controller_pairing: human vs human, human vs ai, ai vs ai
  - relationship: competitive, cooperative, team based, mixed loyalty
  - symmetry: symmetric roles, asymmetric roles and abilities
  - timing: turn based, real time
  - synchronization: command oriented, world state oriented, hybrid
  - visibility: full world, partial, role specific, team only, see concept:visibility-scope
  - composition: human and ai mixed teams
  - observers: spectators, see permission:spectator-receive-only
  - continuity: player replacement mid match, see concept:agent-departure-policy
use: concept:sample-acceptance-matrix maps each value to an executable fixed sample configuration; an uncovered value blocks its claim
independence: axes combine freely, so one sample usually covers several
```
