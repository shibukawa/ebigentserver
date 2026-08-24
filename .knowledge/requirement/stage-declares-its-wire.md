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
removes:
  - the hand-written cborbind wrappers a game used to write, which nothing called
  - the second generate command; ebigent generate drives the codec generator too
  - the //go:generate comment beside every message package
  - concept:cbor-wire-profile and concept:cbor-world-profile as things a game author has to know
resolution:
  discovery: codegen.Stages reads the assertion, follows a local alias, and follows a selector to the package that owns the type
  emission: codegen.EmitAsks writes the ask into a generated file, since v0.5.23 makes calling the entry point the ask
  verification: codegen.CheckAsks compares each generated codec against the struct it came from
  ordering: ask, generate, check, withdraw and regenerate, then delta
landed: world and action
still_out: sight, for the upstream reason below
```

## What the codec generator does instead of failing

tinybind v0.5.23 drops what it cannot encode and says nothing. Bisected one field at a time against a package that otherwise generates, it drops at two grains.

```yaml
whole_type_refused:
  when: a collection whose element type is named
  examples: "[]Mark, [9]Mark — while []uint8 and [9]uint8 are carried"
  symptom: no codec, and the package reports as having nothing to generate
member_dropped:
  when: a member whose type is named in another package
  examples: session.Tick, session.SlotID, fixmath.F64, and transitively any struct embedding one
  symptom: >
    a codec that compiles, round trips, and leaves the member out. This is
    the dangerous one. A world synchronised through it keeps its board and
    loses its tick counter and its RNG state on every send, and the first
    symptom is two peers diverging minutes into a match.
corrections_to_an_earlier_reading:
  - fixed-length arrays are carried; the earlier note blamed the array when the cause was the named element
  - a cross-package named scalar does not refuse the type, it removes the member, which is worse
```

## Why the check refuses rather than trusts

```yaml
rule: a generated codec is compared against its struct before it is kept
how: the container header says how many members the codec writes, the struct says how many it should, and a map codec's keys name which
withdrawn_not_fatal: >
  an ask that fails is removed from the generated file and the generator
  runs again, so the package ends as it was before anything asked — no
  codec, and none that lies. Making it fatal was tried first and stopped
  every project, because a world holds a tick or a seat or a fixed-point
  number.
evidence: >
  the scaffold's own starter world hits the member-dropped mode. Without
  this check every new project would have shipped a State that silently
  loses Tick and Rand.
```

## What is still blocked, and on what

```yaml
sight: >
  hits both modes at once. It names seats with session.SlotID, it carries
  a board of a named cell type, and it delivers session.EvaluationSignal,
  which the framework defines and which holds fixmath.F64.
examples_solo: >
  its world cannot be generated at all, and not only for the upstream
  reason: fixmath.Rand keeps its state in an unexported field, so no
  generated codec can reach it. That one needs cbor.Appender on the type
  itself (rule:shared-rng-seed says the state travels with the world).
asked_upstream: tinybind-cbor-requirements.md section 11
consequence_while_it_stands: >
  a package with a withdrawn ask has its generated file written twice per
  run. The second write restores the first, mtime included, so
  flow:dev-rebuild-loop does not see a change — but the double write goes
  away by itself once nothing is withdrawn.
```
