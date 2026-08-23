---
id: data:decision-record
type: data
title: Decision Record
---
One line of the decisions stream: what an agent could see, what it did, and what followed.

```yaml
fields:
  - episode_id
  - tick, see term:tick
  - agent_id and seat
  - agent_kind, so human and actor:llm-agent rows are separable
  - sight: the concept:sight as delivered, not the world state
  - visibility: data:visibility-annotation
  - action: the concept:action taken, or none
  - evaluation: the data:evaluation-signal at this decision point
  - reward_delta: change since the previous decision, the per decision credit label
  - decision_latency, feeding the reaction_delay axis of concept:behavior-profile
  - outcome_ref, joined to the outcomes stream
sight_is_the_record: logging what concept:agent-view actually delivered is both the cheapest capture point and automatically correct in scope
unit_of_analysis: the segment step of flow:behavior-tree-synthesis consumes exactly these rows
```
