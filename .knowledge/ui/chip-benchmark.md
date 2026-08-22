---
id: ui:chip-benchmark
type: ui
title: Chip Benchmark
---
Developer surface comparing data:agent-loadout personalities by unattended match results, down to per-chip attribution.

```yaml
ui:
  root:
    kind: app
    id: screen.chip-benchmark
    title: Chip Benchmark
    children:
      - kind: matrix
        id: matrix.league
        title: Loadout League
        state: win rate per loadout pairing from concept:continuous-match-loop round_robin or league play
        action: select a cell to drive the panes below
      - kind: panel
        id: panel.attribution
        title: Chip Attribution
        children:
          - kind: table
            id: attribution.chips
            columns:
              - chip
              - fire rate
              - reward_delta correlation
              - presence in wins vs losses
      - kind: panel
        id: panel.ablation
        title: Ablation
        state: re-run the pairing with one chip removed and show the win-rate delta
        action: queue an ablation batch on concept:training-farm
      - kind: chart
        id: chart.tactics
        title: Tactic Frequency
        state: active tactic over time per concept:tactic-selector, from data:decision-record
design_notes:
  - deltas are attributable because every loadout selects from the same data:behavior-chip units, see decision:shared-chip-library
  - aggregates are metric:balance-signals queries over data:episode-log corpora, nothing bespoke
```
