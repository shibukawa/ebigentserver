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
    needs: a reliable ordered stream only, so system:websocket is sufficient and no datagram transport is required at all
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
the_cheapest_case_is_worth_naming: >
  a turn based game needs no unreliable datagram, so it needs neither
  system:webtransport nor system:webrtc. It runs over system:websocket,
  behind any ordinary https endpoint, with no certificate story beyond the
  one a web server already has. Reaching for a datagram transport before
  the game needs one buys deployment cost and nothing else.
```
