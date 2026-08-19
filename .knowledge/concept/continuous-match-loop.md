---
id: concept:continuous-match-loop
type: concept
title: Continuous Match Loop
---
Long-running mode that plays match after match unattended, accumulating a corpus rather than answering one question.

```yaml
difference_from_farm: concept:simulation-farm runs a batch and reports; this runs indefinitely and rotates what it plays
loop:
  - pick a pairing
  - build a data:run-config with that agent roster and a fresh seed
  - run a concept:simulation-mode session to completion
  - append data:episode-log, rotate files, sample or discard low value episodes
  - update running metric:balance-signals
  - repeat
pairing_policies:
  - round_robin over a fixed agent set
  - random sampling over concept:behavior-profile space
  - self_play against the most recently synthesized data:behavior-tree
  - league, keeping past versions as opponents so newer trees cannot forget old counters
diversity_requirement: vary seed, pairing, and profile every match, or the corpus collapses into near identical episodes and adds no information
seed_rule: a fresh seed per match, see rule:shared-rng-seed; a fixed seed makes every match a duplicate
termination: match count, wall time, corpus size, or metric:balance-signals no longer moving
resource_control: corpus growth is the real limit, so sampling and retention are part of the loop, not an afterthought
feeds: flow:behavior-tree-synthesis and flow:behavior-profile-derivation
```
