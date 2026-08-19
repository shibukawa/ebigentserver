---
id: rule:unauthenticated-admission-requires-scope-or-capability
type: rule
title: Unauthenticated Admission Requires Scope Or Capability
---
A session admitting agents without data:session-ticket must be unreachable except by network scope or by holding an invitation.

```yaml
two_valid_controls:
  - name: network_scope
    means: listen only on loopback, private ranges, or link local
    fits: concept:listen-server-mode on a lan, api:lan-discovery
  - name: rendezvous_capability
    means: no listening port exists; a peer can connect only by presenting an offer it was given
    fits: concept:static-host-mode over system:webrtc
    property: the endpoint cannot be scanned or enumerated, unlike an open port
    requires: rule:invitation-is-single-use-and-expiring
default: fail closed; an unauthenticated session on a public listening address is refused at startup
rationale: an unauthenticated concept:listen-server-mode bound to all interfaces is joinable by anyone who finds the port
override: explicit operator opt in, never the default
checked_at: session startup and at flow:session-admission, not only at configuration load
decision: decision:no-auth-on-lan
```
