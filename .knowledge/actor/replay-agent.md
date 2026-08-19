---
id: actor:replay-agent
type: actor
title: Replay Agent
---
concept:agent that emits recorded actions from a stored episode instead of deciding.

```yaml
source: data:episode-log
uses: replay playback, regression test, rollback verification, debugging
requires: term:determinism when reproducing state from actions alone
```
