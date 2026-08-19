---
id: api:message-framing
type: api
title: Message Framing
---
Chunking, reassembly, and backpressure for transports with per-message size limits.

```yaml
need: system:webrtc data channels cap message size, and data:snapshot easily exceeds it
mechanism:
  - split a serialized message into fixed size chunks
  - carry message id, chunk index, and chunk count per frame
  - reassemble only when every chunk of one id has arrived
  - drop malformed frames, out of range indexes, duplicates, and oversized sets before the session layer sees them
bounds:
  chunk_size: about 12 kb, below the smallest practical data channel message limit
  max_message_size: bounded per game, sized from max_snapshot_size of data:session-tuning-profile
  max_pending_reassembly: small fixed count, so a partial flood cannot exhaust memory
  source: hard ceilings from data:runtime-resource-budget
backpressure:
  - serialize sends through a queue
  - pause at a buffered byte high water mark, resume at a low water mark
  - starting points: pause near 256 kb, resume near 64 kb, tuned per tick rate
  - prevents one large snapshot from starving the following ticks
  - shed data:presence-message first, see rule:presence-superseded-not-retransmitted
  - follow policy:overload-handling when the queue remains above its bound
position: between api:transport-interface and api:sequence-ack-layer
applies_to: reliable channels carrying large payloads, not the per tick data:player-input path
untrusted_input: policy:realtime-abuse-protection validates bounds before allocation
```
