---
id: api:manual-signaling-token
type: api
title: Manual Signaling Token
---
Self-contained offer or answer token that players exchange out of band when no signaling server exists.

```yaml
carrier: url fragment, since a fragment is never sent to the http server
forms:
  - invitation: host offer
  - answer: joiner response
ice_mode: non trickle; the token is produced only after ice gathering completes, because there is no channel to trickle over
gathering_timeout: bounded wait around 20 seconds, then emit whatever candidates exist
encoding:
  - version and type header
  - declared payload length, so trailing characters appended by chat or wiki software are not read
  - sdp dictionary substitution, then deflate, then a text safe alphabet
validation: version, reserved bits, declared length, alphabet, inflated size, utf8, json, session id, sdp shape
hygiene: read the fragment into memory at startup, then strip it from the visible url
never_contains: turn credentials, which stay in page memory only
lifetime: short, see rule:invitation-is-single-use-and-expiring
```
