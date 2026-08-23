---
id: requirement:conditional-matchmaking
type: requirement
title: Conditional Matchmaking
---
A room states its terms when it opens, and a joiner is matched by the conjunction of them — never by a host judging each arrival.

```yaml
status: done for api:lan-preset — [[protocol.condition]] declares the axes, generate emits them, the beacon carries a room's terms, and both the browse filter and the seat request run the conjunction
who_decides: >
  the host opens a room with its terms stated and judges nobody after that.
  The decision belongs to whoever arrives, which is how a lobby actually
  works: you see the terms and the members, and you choose. What happens on
  the host side at that moment is a check against what was declared, not an
  opinion — a turnstile rather than a doorman.
two_layers:
  axes: >
    which terms exist at all — rank, mode, map. The game declares them and
    they are protocol level in concept:configuration-scope, generated at
    build, because two ends that disagree about the axis set cannot explain
    a refusal to each other. Same reason a schema is settled at build.
  values: >
    what one room says on those axes. Room level, chosen per launch, so one
    artifact opens a ranked room and a casual one.
matching:
  shape: the conjunction of every axis, and nothing else — no disjunction, no nesting, no expression language
  exact: both sides name a value and they must be equal; either side naming none is no constraint
  band: >
    the room names a range and the joiner brings their own value, which must
    fall inside it. Asymmetric on purpose: a rank is an attribute of the
    player, not a filter they choose, so there is no unset case on their side.
  unset_is_any: an axis a room says nothing about constrains nothing
enforced_where: the places rule:game-version-must-match already runs
  - the discovery beacon, where a room whose terms are not met is left out — display, and not the control
  - the seat request, which is the real gate
refusal_names_the_term: >
  the seat request answers 409 with the axis that failed, and the guest
  surfaces that body rather than the status. A refusal that only says
  conflict defeats the reason for declaring terms at all — the joiner
  cannot tell a full room from a rank band they miss by ten.
not_a_secret: >
  a password is authentication rather than a term, and it collides with
  decision:no-auth-on-lan where scope is the whole of the control. If rooms
  ever need one it is a separate mechanism, not another axis.
beacon_cost: the axes are a rank and a mode or two, which is tens of bytes on a 1400 byte beacon
depends_on: the seat arriving after the room is entered rather than with the connection, see requirement:match-then-sit
