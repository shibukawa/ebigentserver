---
id: flow:behavior-tree-synthesis
type: flow
title: Behavior Tree Synthesis
---
Turn recorded play into an approved behavior tree with the developer deciding what survives.

```yaml
flow:
  trigger: a corpus of data:episode-log exists from human or actor:llm-agent play
  steps:
    - id: curate
      action: filter, aggregate, cap, prioritize, and split the corpus per requirement:corpus-curation
      note: downstream analysis counts one record as one vote, so a raw human corpus distills poorly
    - id: segment
      action: split episodes into decision points, each an concept:sight paired with the concept:action taken
    - id: analyze
      actor: actor:llm-agent
      action: find recurring situation to action patterns and state each as a condition
    - id: vocabulary
      actor: actor:llm-agent
      action: propose data:derived-predicate definitions that name the distinctions the raw fields do not express
      note: predicates come first, so conditions read as judgements rather than coordinate arithmetic
    - id: propose
      output: data:behavior-candidate set, each with rationale, coverage, and concept:behavior-evidence
    - id: rank
      action: order by coverage and reward_delta of data:evaluation-signal, mark conflicts with existing nodes
      note: ranking on terminal outcome alone gives one label per episode, far too sparse to separate good decisions from lucky ones
    - id: review
      actor: developer
      action: accept, edit, or reject each candidate in ui:behavior-tree-editor
    - id: gate
      actor: developer
      action: assign level tags, see concept:skill-level-gating
    - id: commit
      output: accepted candidates graduate into data:behavior-chip entries of the shared library, see rule:generated-behavior-requires-approval and decision:shared-chip-library
    - id: assemble
      actor: developer
      action: select chips into data:agent-loadout personalities, grouped by tactic for concept:tactic-selector
      output: data:behavior-tree per loadout
    - id: generate
      action: emit Go source for the tree and its predicates, see decision:behavior-tree-compiled-to-go
      output: predicate tests from rule:predicate-tests-generated-from-episodes
    - id: validate
      action: run the tree through flow:automated-playtest and compare metric:balance-signals against the source play
  failure:
    condition_uses_hidden_state: reject the candidate, the runtime agent cannot read it
    regression_in_playtest: keep the previous data:behavior-tree version
follows: flow:behavior-profile-derivation, which produces the execution axis rather than the knowledge axis
```
