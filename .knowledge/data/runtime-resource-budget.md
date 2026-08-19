---
id: data:runtime-resource-budget
type: data
title: Runtime Resource Budget
---
Declared hard bounds that keep one game process, session, connection, and decoder within finite resources.

```yaml
process:
  - max_sessions
  - max_connections
  - max_memory_bytes
session:
  - max_agents
  - max_pending_actions
  - tick_cpu_budget
connection:
  - admission_rate
  - input_messages_per_tick
  - input_bytes_per_second
  - send_queue_bytes
decoder:
  - max_message_size
  - max_collection_length
  - max_nesting_depth
  - max_pending_reassembly
shutdown:
  - drain_deadline
  - final_log_flush_deadline
validation: reject missing, zero, contradictory, or allocation unsafe bounds at startup
response: policy:overload-handling
```
