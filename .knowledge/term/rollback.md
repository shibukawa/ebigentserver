---
id: term:rollback
type: term
title: Rollback
---
Predict with local actions, then rewind and resimulate when remote actions arrive late.

```yaml
requires: term:determinism, cheap state save and restore
fits: small player counts, p2p over system:webrtc, fighting games
mode: option of concept:synchronization-mode
```
