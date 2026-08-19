---
id: concept:static-host-mode
type: concept
title: Static Host Mode
---
Ebitengine WASM client served as static files with no backend; one browser hosts the session and peers join over WebRTC.

```yaml
hosting: static html, js, and wasm on any file host or cdn, no application server
topology: concept:listen-server-mode over system:webrtc, the first browser holds authority
shape: star; peers connect to the host only, never to each other
signaling: api:manual-signaling-token, exchanged by the players through any channel they already have
ice: stun still required for address discovery, turn optional for restrictive networks
access_control: possession of the invitation, see rule:invitation-is-single-use-and-expiring
trust: the invited_pair case of concept:trust-model
not_provided:
  - host migration
  - session resume after host loss
  - read only viewing of the last state after disconnect, which a game does not need
limits: one participant per invitation; a new invitation is required to rejoin
```
