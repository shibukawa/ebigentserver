---
id: decision:vm-host-not-for-servers
type: decision
title: Engine Virtual Machine Host Not Used For Servers
---
The experimental host and guest headless facility of system:ebitengine is not a server mechanism; it is a test harness.

```yaml
mechanism_considered: a guest app forwards draw commands over a socket to a host that replays them on a real gpu, driving ticks and injecting input
rejected_for: concept:dedicated-server-mode
rejection_reasons:
  - the host needs a gpu or display, so a deployment gains a rendering process instead of losing one
  - the guest still links the engine, which contradicts the tier a form of concept:engine-coupling-tier
  - desktop only, so it cannot serve the browser roles of requirement:native-and-wasm-targets
  - experimental, version locked between host and guest, and occasionally flaky
already_solved_without_it: a game with separated logic drops the engine at link time, which is cheaper than routing draw commands nowhere
accepted_for: automated client testing, where injected input and captured frames test what requirement:ai-autoplay-testing cannot reach
test_uses:
  - end to end proof that a device press becomes the concept:action api:input-adapter promises
  - golden frames proving rendering is a function of concept:world-state alone
  - acceptance of generated projects beyond compiling, see requirement:project-scaffolding
  - screenshot and video output for flow:automated-playtest and replay review
constraint: desktop only and needs a virtual display in continuous integration, acceptable for tests and not for deployment
```
