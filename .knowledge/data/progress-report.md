---
id: data:progress-report
type: data
title: Progress Report
---
One outbound stream from session to control plane carrying both incremental game facts and the terminal result.

```yaml
unified_on_purpose: an item pickup and a match win are the same shape at different significance; one envelope serves both
fields:
  - session_id and report sequence number, the idempotency key for redelivery
  - tick of occurrence
  - subject: the concept:player-slot involved
  - kind: game defined name, for example item_taken, objective_captured, match_won
  - terminal: true only on the final report, which closes the session record
  - payload: game defined details
promotion: the game decides which data:game-event occurrences are promoted to reports; the framework never guesses significance
consumers: ranking, achievement, quest, match history, all control plane features of concept:control-plane
delivery: reliable, buffered, resent until acknowledged; reports survive brief control plane outages
trust_by_topology:
  dedicated: authoritative, the operator runs the process
  listen_server_and_static_host: the reporter is a player, so reports are forgeable; the control plane decides stakes, for example accepting match history but not ranked results, see concept:trust-model
abandonment: a session that stops reporting without a terminal report is closed as abandoned by the control plane, see decision:host-loss-ends-session
not_this_stream: data:episode-log is the full record for analysis; this stream carries only what the control plane acts on
```
