---
id: concept:realtime-intensity
type: concept
title: Realtime Intensity
---
How much a game's feel depends on the delay between an input and its visible effect. It is an input to almost every other choice rather than a choice of its own.

```yaml
tiers:
  - name: turn_based
    pacing: the session waits for a decision, session.Run rather than RunRealtime
    needs: a reliable ordered stream for the moves themselves; whether a datagram channel is wanted depends on what moves between turns, see simulation_pace_is_not_transport_pace below
    sync: authority is trivial because only one slot acts at a time
    relay_hop: free — nobody notices a hundred milliseconds on a move that took a person ten seconds
    tuning: tick rate is a formality, see data:session-tuning-profile
  - name: paced
    pacing: a wall clock tick, but a frame of delay is invisible
    examples: strategy, simulation, most card and board games with timers
    needs: unreliable datagrams start to pay, since a dropped update is better superseded than retransmitted
    sync: term:server-authority with concept:client-prediction
    relay_hop: affordable
  - name: twitch
    pacing: the input to pixel path is the product
    examples: fighting games, action games, shooters
    needs: unreliable datagrams, a high send rate, and the smallest possible path
    sync: term:rollback or term:delay-buffering, which is why it demands rule:deterministic-simulation-required-for-rollback
    relay_hop: expensive — it is added to every exchange between two players, both ways
decides:
  - the required concept:transport-capability, which is what rule:transport-selected-by-capability then selects on
  - the tick and send rates of data:session-tuning-profile
  - which concept:synchronization-mode is viable
  - whether a server on the data path is affordable, see concept:deployment-combination
simulation_pace_is_not_transport_pace: >
  a turn based simulation does not make the link quiet. Cursors, look
  direction, ping markers, and emotes are data:presence-message: they never
  touch concept:world-state, they travel at their own presence_rate, and
  they are superseded rather than retransmitted
  (rule:presence-superseded-not-retransmitted) — which is datagram
  behaviour. A board game with live cursors wants an unreliable channel
  even though nothing it simulates is realtime.
websocket_only_is_still_a_real_case: >
  a game that shows no live presence at all, only committed moves, needs a
  reliable ordered stream and nothing else. It runs over system:websocket
  behind any ordinary https endpoint, with no certificate story beyond the
  one a web server already has. That case is worth keeping available, but
  it is narrower than "turn based" — it is "nothing moves between turns".
```
