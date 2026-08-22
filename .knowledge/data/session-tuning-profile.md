---
id: data:session-tuning-profile
type: data
title: Session Tuning Profile
---
Declared parameter set that fixes the timing and bandwidth behavior of one game, replacing framework defaults.

```yaml
parameters:
  - tick_rate: simulation steps per second
  - send_rate: network updates per second, often below tick rate
  - snapshot_interval: full data:snapshot cadence
  - max_snapshot_size: sizes the bounds of api:message-framing
  - bandwidth_budget: target bytes per second per receiver
  - baseline_mode: choice from concept:delta-baseline-policy
  - ack_mode: choice from concept:ack-transmission-policy
  - silence_deadline: expressed in missed ticks, not seconds
  - interpolation_delay: client side smoothing budget
  - presence_rate: data:presence-message samples per second, independent of tick rate
  - lag_compensation_window: see concept:lag-compensation, zero disables it
  - history_depth: retained world versions, bounding both delta baselines and lag compensation
consistency_checks:
  - history_depth must cover both the speculation depth and the lag compensation window
  - send_rate times average delta size must fit bandwidth_budget
  - silence_deadline must exceed the worst case gap implied by ack_mode
hard_ceiling: every value must fit data:runtime-resource-budget; tuning cannot raise a process safety bound
presets: shipped as examples per genre, never as defaults, see decision:no-framework-tuning-defaults
measured_against: metric:balance-signals runs and concept:training-farm load tests
```
