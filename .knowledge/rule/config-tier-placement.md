---
id: rule:config-tier-placement
type: rule
title: Config Tier Placement
---
Three questions, asked in order, decide whether a setting is generated, bound, or written in Go.

```yaml
tiers:
  generated: declared in the build sections of ebigent.toml, turned into Go by ebigent build, so the artifact carries a constant and performs no lookup
  bound: declared in the run sections of ebigent.toml, read by configbind at startup per decision:configbind-for-all-config
  literal: written in Go beside the rules, because nothing else can hold it
test:
  - q: can the value be a function, a type, or a closure
    yes: literal
    why: statesync.Codec, session.Config.Validator, eb.Options.Client, and run.Binding.NewAgent are code; TOML has no way to name them
  - q: would changing it require a rebuild anyway
    yes: generated
    why: concept:build-target already fixes it at link time, so a startup lookup only pretends the value is still open, which is the mistake rule:build-tag-only-for-linkage names in its own domain
  - q: may two launches of one artifact legitimately differ
    yes: bound
    no: generated
    why: this is the run tier of data:run-config — an address, a corpus directory, a topology, a roster
placement_of_current_sites:
  generated: run.Options name and devices, run.Binding.ProtocolVersion and EvaluationVersion, session.TuningProfile
  bound: run.RecordOptions root and mode, lan.Options.Port, budget.Budget, the whole of data:run-config
  literal: statesync.Codec, session.Config rule seams, eb.Options.Client and Scene, run.Binding factories
tuning_is_generated_not_bound: >
  data:session-tuning-profile is a game constant under
  decision:no-framework-tuning-defaults, and data:run-config already states
  that a run never overrides it. Declaring it in the build sections and
  emitting it keeps that promise structurally instead of by convention.
defaults_carry_the_rest: >
  a tier answers where a value lives, not whether it must be written. Every
  bound and generated field takes a default, so an entry point states only
  what differs from it. decision:no-framework-tuning-defaults is the single
  exception and it is scoped to the tuning profile.
```
