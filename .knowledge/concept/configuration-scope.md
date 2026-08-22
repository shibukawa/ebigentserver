---
id: concept:configuration-scope
type: concept
title: Configuration Scope
---
The four nesting levels this framework names, from the game's own contract down to one frame on the wire.

```yaml
ladder:
  - level: protocol
    also_known_as: game, which is what the wider industry and system:ebitengine call it
    one_per: project
    holds: seat composition, concept:participant-shape, concept:realtime-intensity, concept:view-arrangement, concept:engine-coupling-tier, accepted devices, the concept:transport-capability the game requires
    seat_composition: >
      seat count, team division, which seats admit a human, a bot, or either,
      and whether an empty seat is refused or filled with a bot. api:roster
      fills what this declares and never adds to it.
    why_the_name: >
      how many play, how they connect, and how tightly the loop must close are
      the game's own contract — the terms every participant agrees to before
      anything runs. That is a protocol in the ordinary sense of the word.
    belongs_in: the build options of ebigent.toml, emitted by ebigent build per rule:config-tier-placement
  - level: match
    one_per: one gathering through one end, concept:match-lifecycle
    holds: api:roster, seed, the link
    implemented_by: run.Match, which runs one session.Session inside itself
  - level: stage
    also_known_as: scene
    one_per: one rule set — a main game, then a bonus round with rules of its own
    holds: a stage rule set, its schema, data:session-tuning-profile
    declared_in_source: the rule set and the schema, given to NewStage as type arguments; ebigent build generates the CBOR codec from them
    bound_at_runtime: data:session-tuning-profile, which every peer of one match must set alike since netplay requires the client to declare the profile the server runs
    per_stage_codec: decision:codec-set-per-stage
  - level: message
    one_per: one frame on the wire
    holds: data:player-input, data:snapshot, data:state-delta, data:game-event
    shaped_by: the schema of the stage that sends it
schema: >
  the message set of one stage, and the term for what an earlier vocabulary
  called a protocol at this level. Protocol names the game's contract; schema
  names the shape of one stage's messages. One stage, one schema.
run_is_not_a_level: >
  where a process listens and how much it may consume are deployment facts,
  not places in this nesting. They travel on the separate axis
  rule:config-tier-placement describes.
what_makes_stage_reachable: >
  the link is already independent of the session — Hosting.Rebind exists
  precisely because a listener outlives a match. What it is not independent of
  is the type: Hosting[S,A,O] can be rebound only to a roster of the same
  triple. Erasing the triple behind a non-generic runner at the match boundary
  is the whole of the change; the transport underneath already carries bytes.
scope_is_not_source: >
  this ladder answers what contains what, rule:config-tier-placement answers
  where a value is written. A protocol-level value is generated; a stage-level
  value can be generated or bound depending on whether the stage set is fixed
  at build.
```
