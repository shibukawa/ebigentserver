---
id: rule:analysis-restricted-to-visible-fields
type: rule
title: Analysis Restricted To Visible Fields
---
Behavior analysis reads only fields marked visible by data:visibility-annotation for that agent at that tick.

```yaml
applies_to: flow:behavior-tree-synthesis, flow:behavior-profile-derivation, and any prompt sent to actor:llm-agent
excluded_by_default: the world ground truth stream of data:episode-log
three_reasons:
  - a condition over hidden state cannot be evaluated at runtime, so the candidate is dead on arrival
  - explaining a decision with information the player did not have produces a confident wrong rationale
  - feeding hidden state to an analyzer trains behavior that looks like cheating
permitted_use_of_ground_truth: outcome labelling and metric:balance-signals, never condition synthesis
enforcement: the analysis query layer joins through the annotation, so the restriction is structural rather than a convention
```
