---
id: decision:no-framework-tuning-defaults
type: decision
title: No Framework Tuning Defaults
---
The framework sets no tick rate, bandwidth budget, or lag compensation policy; each game declares them.

```yaml
decided: yes
rationale: a fighting game, a shooter, a strategy game, and a persistent world disagree on every one of these values, so any default is wrong for most games
instead:
  - collect the parameters in one declared data:session-tuning-profile rather than scattering them across subsystems
  - validate the profile for internal consistency at startup, since the parameters constrain each other
  - ship genre presets as examples, which a game copies and edits
  - measure the result through concept:simulation-farm rather than guessing
consequence: a game must make these choices explicitly, and cannot inherit a silent default that fails at scale
open_by_design: the values stay unanswered in this catalog on purpose; the profile is the answer to where they live
```
