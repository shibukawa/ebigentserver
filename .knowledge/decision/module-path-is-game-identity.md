---
id: decision:module-path-is-game-identity
type: decision
title: Game Identity Pair
---
A game package path carrying its subpath, paired with the schema fingerprint, is what decides whether two peers are the same game.

```yaml
decided: yes
identity: the pair of game package path and data:game-version, compared as one — two peers match when both agree and are strangers otherwise
rule: the path is written out to the game's own package, subpath included, so several games in one module stay distinct
default: project.module of data:build-config, which is already the whole path when a project holds one game — the case flow:project-init produces
emitted: both as build constants, so nothing reads configuration at runtime
declared_not_inferred:
  why: >
    deriving the path from the entry package breaks a game with two entries.
    This repository's pong has cmd/pong and cmd/pong-client, which would carry
    different paths and stop matching each other; a scaffolded project hides
    the problem by giving its client and dedicated targets one shared entry
    under rule:build-tag-only-for-linkage, but that is a convention, not a
    guarantee.
  consequence: one declaration per game, so every target of it emits the same constant whatever its entry
why_version_belongs_in_the_identity:
  closes_a_handshake_hole: >
    data:game-version covers wire-observable shape alone, so two different
    games whose messages happen to have the same shape — two 3x3 board games
    over the same field types — carry the same fingerprint and pass the
    admission handshake today. Comparing the pair refuses that; comparing the
    fingerprint alone cannot detect it.
  one_filter_instead_of_two: >
    a differing identity and a differing version were both already decided to
    hide the beacon, so two checks with one behavior collapse into one
    comparison over one key.
  cost: no build is ever "the same game at another version" — every build is
    its own identity, which is what the decided behavior already meant
title_stays_out_of_it: >
  an earlier form of this put the game name in the identity, which made the
  displayed title unable to be localized or reworded without breaking
  matching. A path carries the disambiguation instead, so run.Options.Name
  goes back to being display alone.
components_stay_separate_on_the_wire: >
  hashing the pair into one opaque value would save bytes and cost the only
  thing the drop log has to say: same game at the wrong build, or a stranger.
  api:lan-discovery keeps them as fields and compares them together.
removes_a_duplication: >
  api:lan-preset asked for a Name of its own, which is why "tictactoe" is
  typed twice in the step2 tutorial. With identity coming from the package
  path and the title from run.Options, lan.Options.Name has nothing left to
  carry — one of the six restatements in concept:config-redundancy closes.
monorepo: >
  the subpath rule is what this buys. A scaffolded project is one module per
  game and needs nothing extra, but this repository's own samples share
  github.com/shibukawa/ebigentserver, and pong and tron would otherwise list
  each other.
beacon_cost: a package path is tens of bytes of the 1400 byte beacon, once a second, and stays readable in a packet capture — worth more than the bytes a hash would save
```
