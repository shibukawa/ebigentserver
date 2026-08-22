---
id: decision:codec-set-per-stage
type: decision
title: Codec Set Per Stage
---
Each stage of a match carries its own generated codec set, so a bonus round encodes nothing like the main game.

```yaml
decided: yes
scope: the stage level of concept:configuration-scope
means: one msg package, one S/A/O triple, one statesync.Codec, and one derived data:game-version per stage, not per game
follows_immediately:
  no_per_stage_version: >
    one fingerprint covering every linked stage is compared once, at match
    time, and nothing per stage is compared or sent. A per-stage version
    would only ever be an intermediate in computing that one value, and
    rule:game-version-must-match already refuses the partial
    compatibility that comparing them separately would imply — agreeing on
    the main game but not the bonus round starts a match that cannot finish.
  wire_carries_a_stage_index_not_a_version: >
    because the fingerprint guarantees both ends hold the identical stage
    list in the identical order, an index resolves to the same codec on both
    sides by construction. It is monotonic, so the same byte also marks a
    packet from a stage already left as stale.
  stage_set_is_declared_not_discovered: >
    a digest computable before play means the stage list is fixed at build.
    So the stage set is game scope even though each stage's content is stage
    scope, and a stage that only runs on a win still counts — both peers link
    its codec either way.
  baseline_history_resets_at_a_boundary: >
    statesync.Sender retains HistoryDepth versions of one state type, so
    crossing into a stage of another type leaves no retained baseline. Every
    stage opens with a snapshot; rule:delta-baseline-must-be-retained is
    satisfied trivially rather than violated.
  tuning_becomes_per_stage: >
    a turn-based main game with a twitch bonus round wants two tick rates, so
    data:session-tuning-profile follows the stage. What stays at game scope is
    the concept:transport-capability requirement, which is the maximum over
    stages: one link serves them all and cannot be renegotiated midway.
  episode_log_spans_versions: >
    data:episode-log records one data:game-version in its header and now
    covers several. Either the header carries the set and every row names its
    stage, or an episode is cut per stage — undecided, and it changes what
    actor:replay-agent reads.
blocking_gap:
  what: statesync.Packet is kind(1), tick(8), baseline(8), payload, with no stage field, and it travels in the datagrams of api:sequence-ack-layer
  why_it_bites: >
    datagrams reorder, so a packet from the previous stage can arrive after
    the receiver switched. concept:cbor-wire-profile encodes fixed-order
    arrays with no field names, so decoding stage N bytes under the stage N+1
    codec is undetectable from the bytes — the exact property
    rule:game-version-must-match cites for refusing to negotiate.
  resolution: >
    one byte of stage index in the packet header. Self-describing, survives
    reordering, and doubles as the staleness test; the frozen layout has to
    move for this release anyway. Coordinating the switch over a reliable
    channel does not substitute — both ends can agree on the current stage
    and a late datagram still arrives after the swap.
named: the level is stage, known outside this framework as a scene; what it defines is a schema, see concept:configuration-scope
```
