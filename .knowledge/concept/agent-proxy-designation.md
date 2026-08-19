---
id: concept:agent-proxy-designation
type: concept
title: Agent Proxy Designation
---
A player nominates which agent takes their seat when they leave or step away.

```yaml
choices:
  - a data:behavior-tree instantiated from the player own concept:behavior-profile
  - a preset profile, for example a cautious or defensive variant
  - none, falling back to the other options of concept:agent-departure-policy
designation_time: at admission, or during play, since stepping away is often intentional rather than a failure
activation: departure detected by api:sequence-ack-layer, or an explicit hand off by the player
return: the human retakes the seat by re-entering, see decision:no-mid-session-reconnect
authorization:
  competitive_case: an authoritative session decides which proxies are permitted, since a designated agent could outperform every human
  mechanism: allow only proxies from a session approved set, validated at admission like any other claim
implementation_cost: none beyond selection, because the seat accepts any concept:agent already
```
