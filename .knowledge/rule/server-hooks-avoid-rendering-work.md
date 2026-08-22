---
id: rule:server-hooks-avoid-rendering-work
type: rule
title: Server Hooks Avoid Rendering Work
---
The arbitrate hook of api:tick-hooks must not create, draw into, or read back images, even in a tier b game of concept:engine-coupling-tier where the engine is linked.

```yaml
applies_to: any hook a server process drives without calling the engine run function
observed: image creation, fill, and draw calls do not crash outside the engine loop, because the engine defers them into a command queue
danger: without a running loop nothing flushes that queue, so the work accumulates instead of failing visibly
forbidden_outright: pixel readback and anything else that must reach a gpu to return a value
allowed: input queries, which return zero values on a server and are the harmless residue of coupled game code
why_not_a_build_check: a tier b server links the engine on purpose, so rule:engine-import-confined-to-client-entry cannot express this and it stays a review rule
tier_a_exempt: separated logic cannot express the mistake, which is one more reason tier a is the recommended destination
```
