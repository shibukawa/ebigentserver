---
id: ui:behavior-tree-editor
type: ui
title: Behavior Tree Editor
---
Developer surface for reviewing generated candidates, editing the tree, and assigning skill levels.

```yaml
ui:
  root:
    kind: app
    id: screen.behavior-editor
    title: Behavior Editor
    children:
      - kind: tree
        id: tree.nodes
        title: Behavior Tree
        state: node status is approved, candidate, or rejected
        action: select a node to drive every other pane
      - kind: panel
        id: panel.candidate
        title: Candidate
        children:
          - kind: text
            id: candidate.condition
            label: Condition
          - kind: text
            id: candidate.rationale
            label: Why the analyzer proposed this
          - kind: metrics
            id: candidate.coverage
            columns:
              - coverage
              - outcome correlation
              - counterexample count
          - kind: button
            label: Accept
            action: rule:generated-behavior-requires-approval
          - kind: button
            label: Reject with reason
            target: data:behavior-candidate
      - kind: panel
        id: panel.evidence
        title: Evidence
        state: shows concept:behavior-evidence for the selected node
        children:
          - kind: table
            id: evidence.situations
            columns:
              - episode
              - tick
              - observation summary
              - action taken
              - outcome
          - kind: table
            id: evidence.counterexamples
            columns:
              - episode
              - tick
              - why the condition matched but the action differed
          - kind: button
            label: Replay this moment
            action: actor:replay-agent
      - kind: panel
        id: panel.predicates
        title: Predicates
        state: the data:derived-predicate vocabulary, reviewed and renamed independently of any node
        children:
          - kind: table
            id: predicate.list
            columns:
              - name
              - observation fields read
              - nodes using it
              - test status
          - kind: text
            id: predicate.body
            label: Generated code
          - kind: table
            id: predicate.cases
            columns:
              - episode
              - tick
              - expected
              - actual
      - kind: matrix
        id: panel.levels
        title: Level Gates
        columns:
          - node
          - beginner
          - intermediate
          - expert
        state: cells toggle the level tags of concept:skill-level-gating
      - kind: panel
        id: panel.diff
        title: Regeneration Diff
        state: appears after a new analysis run, see rule:regeneration-preserves-approved-nodes
        columns:
          - change class
          - node
          - previous decision
          - action
design_notes:
  - the evidence pane is the point of the tool; a condition cannot be judged without the situations it fires in
  - counterexamples sit beside supporting cases, since a rule with high coverage and many counterexamples looks good until they are shown
  - editing a node reveals which levels it affects, the accepted cost of decision:shared-tree-with-level-gates
  - the tree, not this editor, is the artifact; hand editing data:behavior-tree without the editor stays valid
```
