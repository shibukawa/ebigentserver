---
id: system:duckdb
type: system
title: DuckDB
---
Embedded analytical database used to query episode corpora with SQL.

```yaml
fit:
  - reads JSONL, compressed JSONL, and parquet directly from files, with no server and no import step
  - matches the framework habit of shipping artifacts rather than infrastructure
  - handles the aggregate shape of metric:balance-signals naturally
usage: convert a JSONL corpus to parquet once it is queried repeatedly, then scan columns instead of parsing lines
boundary: analysis side only, see rule:analysis-tooling-outside-game-process
go_note: the Go binding needs cgo, which conflicts with wasm builds and with a dependency free game process
optional: the framework defines the data:episode-log schema; using duckdb to read it is a choice
```
