---
id: requirement:ai-autoplay-testing
type: requirement
title: AI Autoplay Testing
---
Framework must run many AI-driven play sessions in parallel as an automated test method.

```yaml
runtime: concept:simulation-farm
flow: flow:automated-playtest
outputs: metric:balance-signals, metric:episode-outcome
agent_mix: varied concept:behavior-profile values, not fixed difficulty classes
harness: the sample suite itself, see decision:samples-as-test-infrastructure
```
