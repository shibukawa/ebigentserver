---
id: requirement:dev-time-inspection
type: requirement
title: Development Time Inspection
---
A developer must see what a running session is doing without adding print statements.

```yaml
why: a game fault is usually a timing, visibility, or decision fault; a log line cannot show a tick budget overrun, a wrong concept:agent-view, or the sight a concept:action came from
surface: ui:dev-console
source: api:dev-debug-endpoint
boundary: development builds only, see rule:debug-endpoint-excluded-from-release
distinct_from: api:runtime-observability, the bounded production signal set that ships in every build
```
