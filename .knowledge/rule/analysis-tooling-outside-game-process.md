---
id: rule:analysis-tooling-outside-game-process
type: rule
title: Analysis Tooling Outside The Game Process
---
The game and session processes write data:episode-log and nothing more; query engines stay in separate tooling.

```yaml
in_process: writing JSONL lines, rotating and compressing files
out_of_process: system:duckdb, parquet conversion, sql queries, notebooks, actor:llm-agent analysis calls
reasons:
  - cgo dependencies would break the wasm target of requirement:native-and-wasm-targets
  - a dedicated server must not carry analytics load beside the tick loop
  - the log schema is the contract, so any query engine remains substitutable
consequence: api:game-cli invokes analysis as a separate step over recorded files, never inside a running concept:session
```
