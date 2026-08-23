---
id: data:derived-predicate
type: data
title: Derived Predicate
---
Named function over concept:sight that turns raw fields into a judgement a behavior condition can use.

```yaml
problem: an observation carries enemy position and own position; a useful condition needs in_range, which no field states
fields:
  - name: the vocabulary term a developer reads, for example in_weapon_range
  - inputs: which concept:sight fields it reads
  - body: generated Go code
  - parameters: thresholds a developer can tune without regenerating
  - evidence: the concept:behavior-evidence situations where it discriminates
  - tests: generated from recorded cases, see rule:predicate-tests-generated-from-episodes
examples: in_weapon_range, outnumbered_locally, escape_route_exists, objective_contested, ally_needs_support
role: the vocabulary layer; conditions in data:behavior-candidate are written over predicates, not raw coordinates
reuse: one predicate serves many nodes, so reviewing it once is cheaper than reviewing the same arithmetic inline everywhere
versioning: changing a predicate invalidates every data:behavior-tree node that reads it
constraints: rule:generated-agent-code-is-deterministic and the observation only limit of rule:analysis-restricted-to-visible-fields
```
