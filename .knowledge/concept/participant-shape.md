---
id: concept:participant-shape
type: concept
title: Participant Shape
---
How many agents share one concept:session and how they reach it — the first question flow:project-init asks, because it is the one that changes generated code rather than a value in a file.

```yaml
shapes:
  - name: solo
    agents: one, plus any local bots
    transport: none, in process only
    signaling: none
    targets: one rendering entry and a simulation entry
  - name: duo
    agents: two
    targets: a client entry, a server entry carrying both linkage forms, and a simulation entry
    reachable_combinations: local_pair, lan_pair_or_party, invited_pair_or_party, brokered_peer, and central, all of concept:deployment-combination
  - name: multi
    agents: many, through one host
    targets: a client entry, a server entry carrying both linkage forms, and a simulation entry
    reachable_combinations: the same set as duo; seat count changes the slot set and admission capacity, not the transport
why_not_topology: concept:execution-topology answers where the session sits, which every shape still chooses at runtime; this answers what has to be generated
why_p2p_is_not_a_shape: peer to peer is star shaped here (concept:static-host-mode), so a peer-hosted session of three is one host and three links, not a structure of its own
why_host_is_not_a_shape: a duo can be player hosted or server hosted just as a multi can, so the host is an orthogonal axis; it is the same server entry built with or without a renderer, see rule:build-tag-only-for-linkage
duo_is_not_serverless: two seats do not imply peer to peer — a ranked one on one wants the central combination, which is the only one whose results carry stakes under concept:trust-model
what_the_shape_actually_decides: the slot set of the generated rules, the admission capacity, and which entries exist; everything else is concept:deployment-combination
```
