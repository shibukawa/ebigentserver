// Package matchloop is concept:continuous-match-loop in its smallest
// form: play match after match unattended with a fresh seed each time,
// and keep running outcome aggregates (metric:episode-outcome — the
// per-slot result and duration; richer metric:balance-signals live in
// SQL over the episode logs). The pairing policy — round robin, random
// sampling, self play, league — is the caller's play function reading
// the match index; the loop owns seeding and aggregation.
package matchloop

import (
	"fmt"

	"github.com/shibukawa/ebigentserver/session"
)

// Result is one finished match.
type Result struct {
	// Outcomes is each slot's terminal signal.
	Outcomes map[session.SlotID]session.Terminal
	// Ticks is the match duration.
	Ticks session.Tick
}

// Summary aggregates a run.
type Summary struct {
	Matches int
	// BySlot counts terminals per slot: BySlot[slot][terminal].
	BySlot map[session.SlotID]map[session.Terminal]int
	// TotalTicks sums durations; divide for the mean.
	TotalTicks uint64
}

// WinRate reports a slot's share of wins.
func (s Summary) WinRate(slot session.SlotID) float64 {
	if s.Matches == 0 {
		return 0
	}
	return float64(s.BySlot[slot][session.Win]) / float64(s.Matches)
}

// Run plays n matches. Each gets a distinct seed derived from baseSeed
// (rule:shared-rng-seed per session; a fixed seed would make every match
// a duplicate and the corpus worthless).
func Run(n int, baseSeed uint64, play func(match int, seed uint64) (Result, error)) (Summary, error) {
	sum := Summary{BySlot: map[session.SlotID]map[session.Terminal]int{}}
	for i := 0; i < n; i++ {
		res, err := play(i, splitmix64(baseSeed+uint64(i)))
		if err != nil {
			return sum, fmt.Errorf("matchloop: match %d: %w", i, err)
		}
		sum.Matches++
		sum.TotalTicks += uint64(res.Ticks)
		for slot, term := range res.Outcomes {
			if sum.BySlot[slot] == nil {
				sum.BySlot[slot] = map[session.Terminal]int{}
			}
			sum.BySlot[slot][term]++
		}
	}
	return sum, nil
}

// splitmix64 spreads sequential bases into unrelated seeds.
func splitmix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}
