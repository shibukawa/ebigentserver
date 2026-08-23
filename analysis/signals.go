package analysis

import (
	"fmt"
	"io"
	"sort"
)

// Report is the pure-Go subset of metric:balance-signals computed over a
// corpus, anchored on metric:episode-outcome rows: outcome counts per
// slot and per agent kind, the duration distribution, decision/action
// frequency, rejection counts, and per-stream storage totals. Richer
// signals (path clustering, tactic mining) stay in SQL over the same
// files — see WriteDuckDBSQL and system:duckdb.
type Report struct {
	// Episodes is the corpus size.
	Episodes int
	// ByProtocol counts episodes per data:protocol-version (the
	// header's protocol_version; "" groups as "(none)").
	ByProtocol map[string]int
	// ByMode counts episodes per concept:episode-recording-mode
	// (replay_complete vs analysis_sampled).
	ByMode map[string]int
	// ResultsBySlot counts terminal results per slot:
	// ResultsBySlot[slot][result].
	ResultsBySlot map[uint16]map[string]int
	// ResultsByAgentKind counts terminal results per agent kind,
	// joining each episode's decisions-stream slot→kind table with
	// its outcomes. Slots with no recorded kind group as
	// "(unknown)". This is the win-rate-by-agent-profile signal of
	// metric:balance-signals.
	ResultsByAgentKind map[string]map[string]int
	// Duration is the episode-length distribution in ticks.
	Duration DurationStats
	// DecisionRows counts data rows across all decisions streams.
	DecisionRows int
	// DecisionsPerEpisodeMean is DecisionRows / Episodes.
	DecisionsPerEpisodeMean float64
	// ActionRows counts rows where the slot acted; SightRows
	// counts sight-only deliveries (null action).
	ActionRows int
	SightRows  int
	// Rejections ranks validator rejection reasons (from the events
	// stream) by count.
	Rejections map[string]int
	// Checkpoints totals data:state-checkpoint events.
	Checkpoints int
	// StreamBytes totals on-disk bytes per stream name — the
	// operations number behind retention and transfer planning.
	StreamBytes map[string]int64
}

// DurationStats summarizes episode duration in ticks.
type DurationStats struct {
	MinTicks  uint64
	MaxTicks  uint64
	MeanTicks float64
	// Histogram uses the fixed buckets of durationBuckets, so two
	// corpora are always comparable bucket for bucket.
	Histogram []HistogramBucket
}

// HistogramBucket is one fixed duration bucket.
type HistogramBucket struct {
	Label string
	Count int
}

// durationBuckets are the fixed histogram edges (upper bounds,
// exclusive). Fixed rather than data-derived so reports over different
// corpora line up.
var durationBuckets = []struct {
	Label string
	Max   uint64
}{
	{"0-9", 10},
	{"10-24", 25},
	{"25-49", 50},
	{"50-99", 100},
	{"100-249", 250},
	{"250+", ^uint64(0)},
}

// Compute derives the Report from a loaded corpus. It is deterministic:
// same corpus, same report.
func Compute(c *Corpus) Report {
	r := Report{
		ByProtocol:         map[string]int{},
		ByMode:             map[string]int{},
		ResultsBySlot:      map[uint16]map[string]int{},
		ResultsByAgentKind: map[string]map[string]int{},
		Rejections:         map[string]int{},
		StreamBytes:        map[string]int64{},
	}
	r.Duration.Histogram = make([]HistogramBucket, len(durationBuckets))
	for i, b := range durationBuckets {
		r.Duration.Histogram[i].Label = b.Label
	}
	var durSum, durCount uint64
	for _, ep := range c.Episodes {
		r.Episodes++
		proto := ep.Header.ProtocolVersion
		if proto == "" {
			proto = "(none)"
		}
		r.ByProtocol[proto]++
		r.ByMode[string(ep.Header.Mode)]++

		kinds := ep.AgentKinds()
		for _, o := range ep.Outcomes {
			bump(r.ResultsBySlot, o.Slot, o.Result)
			kind, ok := kinds[o.Slot]
			if !ok {
				kind = "(unknown)"
			}
			bump(r.ResultsByAgentKind, kind, o.Result)
		}
		if len(ep.Outcomes) > 0 {
			d := ep.Outcomes[0].DurationTicks
			if durCount == 0 || d < r.Duration.MinTicks {
				r.Duration.MinTicks = d
			}
			if d > r.Duration.MaxTicks {
				r.Duration.MaxTicks = d
			}
			durSum += d
			durCount++
			for i, b := range durationBuckets {
				if d < b.Max {
					r.Duration.Histogram[i].Count++
					break
				}
			}
		}

		r.DecisionRows += len(ep.Decisions)
		for _, d := range ep.Decisions {
			if d.HasAction {
				r.ActionRows++
			} else {
				r.SightRows++
			}
		}
		for reason, n := range ep.Rejections {
			r.Rejections[reason] += n
		}
		r.Checkpoints += ep.Checkpoints
		for stream, n := range ep.StreamBytes {
			r.StreamBytes[stream] += n
		}
	}
	if durCount > 0 {
		r.Duration.MeanTicks = float64(durSum) / float64(durCount)
	}
	if r.Episodes > 0 {
		r.DecisionsPerEpisodeMean = float64(r.DecisionRows) / float64(r.Episodes)
	}
	return r
}

func bump[K comparable](m map[K]map[string]int, key K, result string) {
	if m[key] == nil {
		m[key] = map[string]int{}
	}
	m[key][result]++
}

// resultOrder fixes the display order of terminal results; anything
// unexpected sorts after these, alphabetically.
var resultOrder = map[string]int{"win": 0, "lose": 1, "draw": 2, "abandoned": 3}

func sortedResults(counts map[string]int) []string {
	keys := sortedKeys(counts)
	sort.SliceStable(keys, func(i, j int) bool {
		oi, iok := resultOrder[keys[i]]
		oj, jok := resultOrder[keys[j]]
		switch {
		case iok && jok:
			return oi < oj
		case iok:
			return true
		case jok:
			return false
		default:
			return keys[i] < keys[j]
		}
	})
	return keys
}

func writeResultLine(w io.Writer, label string, counts map[string]int) {
	fmt.Fprintf(w, "  %s:", label)
	for _, res := range sortedResults(counts) {
		fmt.Fprintf(w, " %s=%d", res, counts[res])
	}
	fmt.Fprintln(w)
}

// WriteText renders the report as a plain-text table with fully
// deterministic ordering (every map is emitted in sorted key order).
func (r Report) WriteText(w io.Writer) {
	fmt.Fprintln(w, "episode corpus report (metric:episode-outcome, metric:balance-signals)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "episodes: %d\n", r.Episodes)

	fmt.Fprintln(w, "by protocol version:")
	for _, k := range sortedKeys(r.ByProtocol) {
		fmt.Fprintf(w, "  %s: %d\n", k, r.ByProtocol[k])
	}
	fmt.Fprintln(w, "by recording mode:")
	for _, k := range sortedKeys(r.ByMode) {
		fmt.Fprintf(w, "  %s: %d\n", k, r.ByMode[k])
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "outcomes by slot:")
	slots := make([]int, 0, len(r.ResultsBySlot))
	for slot := range r.ResultsBySlot {
		slots = append(slots, int(slot))
	}
	sort.Ints(slots)
	for _, slot := range slots {
		writeResultLine(w, fmt.Sprintf("slot %d", slot), r.ResultsBySlot[uint16(slot)])
	}
	fmt.Fprintln(w, "outcomes by agent kind:")
	for _, kind := range sortedKeys(r.ResultsByAgentKind) {
		writeResultLine(w, kind, r.ResultsByAgentKind[kind])
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "duration ticks: min=%d mean=%.2f max=%d\n",
		r.Duration.MinTicks, r.Duration.MeanTicks, r.Duration.MaxTicks)
	fmt.Fprintln(w, "duration histogram:")
	for _, b := range r.Duration.Histogram {
		fmt.Fprintf(w, "  %s: %d\n", b.Label, b.Count)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "decision rows: %d (mean %.2f per episode)\n",
		r.DecisionRows, r.DecisionsPerEpisodeMean)
	fmt.Fprintf(w, "  with action: %d\n", r.ActionRows)
	fmt.Fprintf(w, "  sight-only: %d\n", r.SightRows)
	fmt.Fprintf(w, "checkpoints: %d\n", r.Checkpoints)

	fmt.Fprintln(w, "rejection reasons:")
	if len(r.Rejections) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		// Ranked by count, ties alphabetical.
		reasons := sortedKeys(r.Rejections)
		sort.SliceStable(reasons, func(i, j int) bool {
			return r.Rejections[reasons[i]] > r.Rejections[reasons[j]]
		})
		for _, reason := range reasons {
			fmt.Fprintf(w, "  %d  %s\n", r.Rejections[reason], reason)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "stream bytes:")
	var total int64
	for _, k := range sortedKeys(r.StreamBytes) {
		fmt.Fprintf(w, "  %s: %d\n", k, r.StreamBytes[k])
		total += r.StreamBytes[k]
	}
	fmt.Fprintf(w, "  total: %d\n", total)
}
