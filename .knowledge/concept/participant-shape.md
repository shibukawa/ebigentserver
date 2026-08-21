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
    targets: follow from concept:view-arrangement, not from the seat count
  - name: multi
    agents: many
    targets: follow from concept:view-arrangement, not from the seat count
why_not_topology: concept:execution-topology answers where the session sits, which every shape still chooses at runtime; this answers what has to be generated
why_p2p_is_not_a_shape: peer to peer is star shaped here (concept:static-host-mode), so a peer-hosted session of three is one host and three links, not a structure of its own
why_host_is_not_a_shape: a duo can be player hosted or server hosted just as a multi can, so the host is an orthogonal axis; it is the same server entry built with or without a renderer, see rule:build-tag-only-for-linkage
seats_do_not_imply_a_network: two or twenty seats in one process are concept:standalone-mode with no transport at all; the process boundary is what decides whether a link exists, and only then do concept:synchronization-mode and the host question arise
camera_is_a_third_axis: concept:view-arrangement is independent of both — an online fighting game shares a camera across separate processes, and a split screen console game splits one inside a single process
duo_is_not_serverless: two seats do not imply peer to peer — a ranked one on one wants the central combination, which is the only one whose results carry stakes under concept:trust-model
what_the_shape_actually_decides: the slot set of the generated rules, the admission capacity, and which entries exist; everything else is concept:deployment-combination
```
