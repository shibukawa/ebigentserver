---
id: rule:evaluation-computed-by-session
type: rule
title: Evaluation Computed By The Session
---
data:evaluation-signal is produced by concept:session and never by a client or an agent.

```yaml
reasons:
  - an agent scoring itself can be gamed, and two agents would disagree about the same position
  - runtime decisions and later analysis must read identical numbers, or learning optimizes a different target than play
  - it is derived from concept:world-state, which only the authoritative side holds
recording: written into every data:decision-record, not recomputed during analysis
recompute_exception: replaying a corpus under a new evaluation function is allowed, but produces a new versioned corpus rather than editing the old one
sparse_versus_dense: a terminal only signal gives one label per episode; intermediate signals are what make per decision credit assignment possible in flow:behavior-tree-synthesis
```
