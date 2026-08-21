package cli

import "testing"

// A pinned toolchain is only worth pinning if doctor notices when the
// host does not meet it.
func TestToolchainPinComparison(t *testing.T) {
	if got := goVersion("go version go1.26.7 darwin/arm64\n"); got != "1.26.7" {
		t.Errorf("goVersion = %q, want 1.26.7", got)
	}
	for _, tc := range []struct {
		have, want string
		older      bool
	}{
		{"1.26.7", "1.26.5", false}, // a newer patch satisfies the pin
		{"1.26.5", "1.26.5", false},
		{"1.26.4", "1.26.5", true},
		{"1.25.9", "1.26.0", true},
		{"1.26", "1.26.5", true}, // an unset patch reads as zero
		{"1.27", "1.26.5", false},
		{"weird", "1.26.5", false}, // unusable comparisons must not fail a run
	} {
		if got := olderThan(tc.have, tc.want); got != tc.older {
			t.Errorf("olderThan(%q, %q) = %v, want %v", tc.have, tc.want, got, tc.older)
		}
	}
}
