package run_test

import (
	"strings"
	"testing"

	"github.com/shibukawa/ebigentserver/run"
)

// axes is a game declaring one of each kind: a mode you pick and a rank
// you have.
func axes() []run.Axis {
	return []run.Axis{{Name: "mode"}, {Name: "rank", Band: true}}
}

// TestTermsAreTheConjunction covers the whole of matchmaking's logic: a
// joiner sits when every axis is satisfied and not otherwise. There is no
// disjunction to test because there is none to have.
func TestTermsAreTheConjunction(t *testing.T) {
	room := run.Terms{
		Exact: map[string]string{"mode": "ranked"},
		Low:   map[string]int{"rank": 1200},
		High:  map[string]int{"rank": 1600},
	}
	cases := []struct {
		name  string
		wants run.Wants
		deny  string
	}{
		{
			"both axes met",
			run.Wants{Exact: map[string]string{"mode": "ranked"}, Value: map[string]int{"rank": 1400}},
			"",
		},
		{
			"the exact axis differs",
			run.Wants{Exact: map[string]string{"mode": "casual"}, Value: map[string]int{"rank": 1400}},
			"mode",
		},
		{
			"below the band",
			run.Wants{Exact: map[string]string{"mode": "ranked"}, Value: map[string]int{"rank": 900}},
			"rank",
		},
		{
			"above the band",
			run.Wants{Exact: map[string]string{"mode": "ranked"}, Value: map[string]int{"rank": 2000}},
			"rank",
		},
		{
			// The edges are inside: a room admitting 1200..1600 admits
			// somebody at exactly 1200.
			"on the low edge",
			run.Wants{Exact: map[string]string{"mode": "ranked"}, Value: map[string]int{"rank": 1200}},
			"",
		},
		{
			"on the high edge",
			run.Wants{Exact: map[string]string{"mode": "ranked"}, Value: map[string]int{"rank": 1600}},
			"",
		},
		{
			// Not caring about the mode is not a reason to refuse.
			"asking nothing on the exact axis",
			run.Wants{Value: map[string]int{"rank": 1400}},
			"",
		},
		{
			// A band is an attribute, so there is no not-caring: a room
			// that bounded one needs the reading.
			"bringing no reading for a band",
			run.Wants{Exact: map[string]string{"mode": "ranked"}},
			"rank",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := room.Satisfies(axes(), tc.wants)
			switch {
			case tc.deny == "" && err != nil:
				t.Fatalf("refused a joiner it should admit: %v", err)
			case tc.deny != "" && err == nil:
				t.Fatal("admitted a joiner it should refuse")
			case tc.deny != "" && !strings.Contains(err.Error(), tc.deny):
				t.Errorf("refusal %q does not name %s", err, tc.deny)
			}
		})
	}
}

// TestATermlessRoomAdmitsAnybody keeps the axes from becoming a tax on
// games that declare none: an axis a room says nothing about constrains
// nothing.
func TestATermlessRoomAdmitsAnybody(t *testing.T) {
	if err := (run.Terms{}).Satisfies(axes(), run.Wants{}); err != nil {
		t.Fatalf("a room stating nothing refused somebody: %v", err)
	}
	picky := run.Wants{Exact: map[string]string{"mode": "ranked"}, Value: map[string]int{"rank": 1400}}
	if err := (run.Terms{}).Satisfies(axes(), picky); err != nil {
		t.Fatalf("a room stating nothing refused a joiner who asked: %v", err)
	}
	// And a game with no axes at all has nothing to check.
	if err := (run.Terms{Exact: map[string]string{"mode": "ranked"}}).Satisfies(nil, run.Wants{}); err != nil {
		t.Fatalf("a game with no axes refused somebody: %v", err)
	}
}

// TestDescribeReadsTheSameForEveryRoom keeps a browse list from ordering
// two rooms' terms differently just because their maps iterated apart.
func TestDescribeReadsTheSameForEveryRoom(t *testing.T) {
	room := run.Terms{
		Exact: map[string]string{"mode": "ranked"},
		Low:   map[string]int{"rank": 1200},
		High:  map[string]int{"rank": 1600},
	}
	first := room.Describe(axes())
	for range 20 {
		if got := room.Describe(axes()); got != first {
			t.Fatalf("described %q then %q", first, got)
		}
	}
	if !strings.Contains(first, "mode ranked") || !strings.Contains(first, "rank 1200..1600") {
		t.Errorf("description %q does not carry both axes", first)
	}
}
