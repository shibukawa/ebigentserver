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
    seats: fixed at one
    link: none, in process only
    targets: one rendering entry and a simulation entry
  - name: duo
    agents: two
    seats: fixed at two
    targets: a client entry, a server entry carrying both linkage forms, and a simulation entry
    why_its_own_shape: two is the one seat count where a peer link is genuinely one hop, since the host is the other player; that is what puts term:rollback and term:delay-buffering in reach
  - name: multi
    agents: as many as the project declares
    seats: asked, since nothing fixes it
    targets: the same set duo generates — the seat count changes the slot set and the admission capacity, not the wiring
why_not_topology: concept:execution-topology answers where the session sits, which every shape still chooses at runtime; this answers what has to be generated
why_p2p_is_not_a_shape: peer to peer is star shaped here (concept:static-host-mode), so a peer-hosted session of three is one host and three links, not a structure of its own
why_host_is_not_a_shape: a duo can be player hosted or server hosted just as a multi can, so the host is an orthogonal axis; it is the same server entry built with or without a renderer, see rule:build-tag-only-for-linkage
what_decides_the_targets: whether a link exists, which follows from the shape — solo has nowhere to reach, and everything else generates a client and a server
why_seats_reach_generated_code_at_all: not because two seats wire differently from six, but because past two every exchange is two hops through a host either way, so the one-hop netcodes stop being reachable; see concept:deployment-combination
camera_is_a_separate_axis: concept:view-arrangement is independent — an online fighting game shares a camera across separate machines, and a split screen console game splits one inside a single process. What flow:project-init asks is the code visible half of it, whether a machine may seat several players
duo_is_not_serverless: two seats do not imply peer to peer — a ranked one on one wants the central combination, which is the only one whose results carry stakes under concept:trust-model
what_the_shape_actually_decides: the slot set of the generated rules, the admission capacity, and which entries exist; everything else is concept:deployment-combination
```
