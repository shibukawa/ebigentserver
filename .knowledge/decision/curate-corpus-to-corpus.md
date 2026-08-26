---
id: decision:curate-corpus-to-corpus
type: decision
title: Curate Is A Corpus To Corpus Transform
---
The curate stage of requirement:corpus-curation reads a recorded corpus and writes a curated one, leaving segment and mining untouched.

```yaml
decision: curate transforms data:episode-log directories into a data:curated-corpus before behavior.Segment runs
rationale:
  vocabulary_independence: dedup keys on the canonical recorded sight, not on feature bits; a weak vocabulary aliases distinct situations and would merge counts wrongly
  inspectable_stages: the intermediate stays JSONL a person can open and diff, like every other pipeline artifact
  downstream_untouched: Segment, SequentialCovering, Merge, and codegen consume the curated corpus unchanged
mechanics:
  situation_key: canonical sight JSON per slot, with per-situation action counts retained
  cap_by_copies: keep min(count, cap) rows per situation-action pair, ordered by episode and tick, so coverage keeps meaning "supporting records" and Evidence stays replayable
  conflicts_surfaced_not_resolved: a situation carrying several actions keeps them all and is listed in the report; no automatic majority vote, per rule:generated-behavior-requires-approval
  episode_split: deterministic hash of episode id and seed assigns whole episodes to train or holdout, preventing near-duplicate leakage across the split
  gap_detection: behavior.Evaluate replays approved chips against holdout rows; silent situations become the collection list for the next simulate round
  filters: agent_kind, seat, and outcome via a row filter shared with Segment, whose seat-only keep func widens to the full data:decision-record row
rejected:
  weighted_records: a Weight field on mining records changes coverage semantics everywhere for a gain the cap already provides
  in_memory_only: curating featurized records ties dedup to the current vocabulary and hides the stage from inspection
```
