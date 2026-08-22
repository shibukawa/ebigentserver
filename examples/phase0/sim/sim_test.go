package sim

import "testing"

// The digest is pinned: it freezes fixmath's output bits, the wire scales,
// and the generated codec bytes together. If this test breaks, either a
// protocol-relevant bit moved (bump data:protocol-version and repin
// deliberately) or a regression changed simulation output (fix it).
// Moved once, when concept:cbor-world-profile became the map shape of
// tinybind v0.5.23 and members began carrying their names instead of
// integer labels. An episode digest covers world state, so it followed.
const pinnedDigest = 0x073ace847b778cc8

func TestEpisodeDigestIsReproducible(t *testing.T) {
	first := Run(1000)
	second := Run(1000)
	if first != second {
		t.Fatalf("two runs diverged: %016x vs %016x", first, second)
	}
}

func TestEpisodeDigestIsPinned(t *testing.T) {
	got := Run(1000)
	if got != pinnedDigest {
		t.Fatalf("episode digest = %016x, pinned %016x — a protocol-relevant bit moved", got, pinnedDigest)
	}
}
