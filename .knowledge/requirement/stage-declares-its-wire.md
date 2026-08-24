---
id: requirement:stage-declares-its-wire
type: requirement
title: Stage Declares Its Wire
---
The rule set declaration is the one place a stage meets the framework, so everything generation needs is read from it and nowhere else.

```yaml
declaration: StageRuleSet[World, Action, Sight] — the assertion a game writes anyway to say its rules satisfy the contract
what_each_position_asks_for:
  world: a whole codec and a delta, since it synchronises and only the change travels each tick
  action: a whole codec; it is one small message going upstream and has no baseline to diff against
  sight: a whole codec; it reaches an agent in another process and it is what data:episode-log records per decision
everything_serialises: >
  including a solo game. It has no link, but its sights and actions are
  what concept:behavior-distillation reads back, so a game with no peer
  still encodes. Treating local play as needing no codec was the mistake
  that made an earlier attempt hunt for extra signals — there are none to
  hunt for, because the rule set already said everything.
removes: the statesync.Codec and delta-reference signals an earlier attempt needed, and the hand-written cborbind wrappers a game writes today
blocked_on_upstream:
  what: tinybind-go v0.5.23 drops a struct it cannot fully encode instead of refusing it, and reports the package as having nothing to generate
  cases:
    - a fixed-length array; slices are carried and arrays are not
    - a named scalar declared in another package, such as session.SlotID
    - a slice of a named scalar, such as []Mark, even in the same package
    - and so, transitively, any struct embedding one — session.EvaluationSignal carries fixmath.F64, which is why a sight cannot be generated today
  why_it_bites_here: >
    a sight is exactly the shape that hits all three. It names seats, it
    carries a board of a named cell type, and it delivers the evaluation
    signal the framework defines. Until the upstream refusal is loud and
    the field types are carried, the sight half of this requirement cannot
    land.
  silence_is_the_worst_part: >
    a dropped type produces no error and no codec. What the caller sees is
    "nothing to generate" for the whole package, which names neither the
    type nor the field.
verified: >
  same-package named scalars and plain slices generate; cross-package
  named scalars, slices of named scalars, and structs embedding either do
  not. Bisected one field at a time against a package that otherwise
  generates.
