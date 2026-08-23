---
id: requirement:match-then-sit
type: requirement
title: Match Then Sit
---
Reaching a room and taking a seat are two acts, because a player may see who is there and leave.

```yaml
status: done for api:lan-preset — /room publishes the roster, /sit takes a seat, and ui:lobby-scene shows the room between them
problem: >
  the seat is granted inside the join handshake today, so entering a room and
  occupying one of its seats are the same call. A player who looks at the
  roster and thinks better of it has already taken a seat and given it back,
  which every other player watched happen.
states:
  discovered: a row in the browse list, no connection
  matched: connected to the room, the roster visible, holding no seat
  seated: holding a seat, playing when the match starts
  leaving_costs: nothing from matched, one freed seat from seated
how: >
  the host serves /room, which grants nothing and costs nothing, so a
  guest can read the roster and leave. /sit is the separate call that
  claims a seat and mints the ticket. A matched guest re-reads the room
  once a second, so somebody arriving shows up while the player is still
  deciding rather than after.
ticket_is_minted_at_sit: >
  the ticket still names a seat, and that is now correct rather than
  premature: it is issued by the check that seated you, so it is proof
  the terms were met and which seat they got you.
one_verb_for_seating: >
  sitting is the same act wherever the person is, so JoinLocal and JoinRemote
  stop being different verbs. Which side of a link they sat from is reported
  on the seat, not declared by the caller — the same split concept:configuration-scope
  already applies to Seat.Local.
ticket_means_something_else: >
  data:session-ticket carries a seat today. Split, it carries proof that the
  declared terms were satisfied — the result of a check rather than the record
  of a decision, which is what requirement:conditional-matchmaking needs it to be.
touches: flow:session-admission, so this is a design change rather than a rename
