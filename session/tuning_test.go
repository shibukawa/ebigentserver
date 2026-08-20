package session_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/session"
)

func TestTuningValidation(t *testing.T) {
	valid := session.TuningProfile{TickRate: 60, SendRate: 20, HistoryDepth: 8}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
	if got := valid.SendEvery(); got != 3 {
		t.Fatalf("SendEvery = %d, want 3", got)
	}

	cases := []struct {
		name string
		p    session.TuningProfile
	}{
		{"zero value (no defaults)", session.TuningProfile{}},
		{"missing tick rate", session.TuningProfile{SendRate: 20, HistoryDepth: 1}},
		{"missing send rate", session.TuningProfile{TickRate: 60, HistoryDepth: 1}},
		{"missing history depth", session.TuningProfile{TickRate: 60, SendRate: 20}},
		{"send above tick", session.TuningProfile{TickRate: 30, SendRate: 60, HistoryDepth: 1}},
		{"non-multiple rates", session.TuningProfile{TickRate: 60, SendRate: 25, HistoryDepth: 1}},
		{"negative field", session.TuningProfile{TickRate: 60, SendRate: 20, HistoryDepth: 1, SnapshotEvery: -1}},
	}
	for _, c := range cases {
		if err := c.p.Validate(); err == nil {
			t.Errorf("%s: expected validation error", c.name)
		}
	}
}
