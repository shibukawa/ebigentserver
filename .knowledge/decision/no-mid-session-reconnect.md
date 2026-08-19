---
id: decision:no-mid-session-reconnect
type: decision
title: No Mid Session Reconnect
---
There is no connection resume; a dropped agent has departed, and returning is a fresh admission.

```yaml
decided: yes
rejected_alternative: connection resume with buffered sequences and catch up replay
rejection_reasons:
  - it requires holding per connection send history for an unknown duration, on top of the baselines of rule:delta-baseline-must-be-retained
  - short match sessions rarely outlive the disconnect, so the machinery is built for a case that mostly ends anyway
  - persistent worlds do not need it either; re-entering is a normal join against a session that never stopped
mechanism_for_return: run flow:session-admission again and receive a full data:snapshot
ticket_consequence: the original data:session-ticket is spent, so a new one is required, see rule:invitation-is-single-use-and-expiring
no_issuer_case: concept:static-host-mode and lan play need a new invitation, since no control plane can mint one
what_remains: concept:agent-departure-policy decides what happens to the seat in the meantime
```
