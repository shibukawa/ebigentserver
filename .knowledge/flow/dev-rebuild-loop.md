---
id: flow:dev-rebuild-loop
type: flow
title: Dev Rebuild Loop
---
What the dev verb does on start and on every source change.

```yaml
start:
  - resolve data:build-config by walking upward from the working directory
  - run tinybind codegen
  - build the configured dev target
  - start it with data:run-config values as CLI options and api:dev-debug-endpoint enabled
  - serve ui:dev-console and attach it to that endpoint
on_change:
  - debounce, then regenerate only when a codegen source changed
  - rebuild; on failure keep the running process alive and surface the error in the console
  - on success stop the child, restart it, and reattach the console
argument_split: ebigent and the child both bind --config-path and other configbind options, so arguments after -- pass through to the child untouched
stop: Ctrl-C stops child processes before the console, in reverse start order
session_continuity: a restart ends the session rather than resuming it, since concept:session-lifecycle has no resume under decision:no-mid-session-reconnect
serves: requirement:live-dev-loop
```
