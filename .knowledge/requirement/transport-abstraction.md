---
id: requirement:transport-abstraction
type: requirement
title: Transport Abstraction
---
Agent-to-session communication must be selectable at runtime through one transport interface.

```yaml
interface: api:transport-interface
selection_basis: concept:transport-capability, not protocol name
rule: rule:transport-selected-by-capability
implementations:
  - local in-process
  - system:websocket
  - system:webtransport
  - system:webrtc
  - system:quic-udp
```
