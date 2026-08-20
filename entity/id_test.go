package entity_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/entity"
)

func TestComposeRoundTrip(t *testing.T) {
	cases := []struct {
		owner uint16
		seq   uint64
	}{
		{0, 0},         // session namespace, first entity
		{1, 0},         // slot 1
		{65535, 1},     // max namespace
		{7, 1<<48 - 1}, // max sequence
		{3, 123456789}, //
	}
	for _, c := range cases {
		id := entity.Compose(c.owner, c.seq)
		if id.Owner() != c.owner || id.Seq() != c.seq {
			t.Errorf("Compose(%d, %d) round-trips as (%d, %d)", c.owner, c.seq, id.Owner(), id.Seq())
		}
	}
}

func TestSequenceMasksTo48Bits(t *testing.T) {
	id := entity.Compose(1, 1<<48+5)
	if id.Owner() != 1 || id.Seq() != 5 {
		t.Errorf("overflowing seq: owner=%d seq=%d, want 1, 5", id.Owner(), id.Seq())
	}
}

// Two owners can never produce the same ID, by construction.
func TestNamespacesNeverCollide(t *testing.T) {
	a := entity.Allocator{OwnerID: 1}
	b := entity.Allocator{OwnerID: 2}
	seen := map[entity.ID]bool{}
	for range 1000 {
		for _, id := range []entity.ID{a.Next(), b.Next()} {
			if seen[id] {
				t.Fatalf("duplicate ID %d", id)
			}
			seen[id] = true
		}
	}
}

// Replaying the same allocation steps regenerates identical IDs — what
// actor:replay-agent depends on.
func TestAllocationIsReproducible(t *testing.T) {
	run := func() []entity.ID {
		a := entity.Allocator{OwnerID: 9}
		ids := make([]entity.ID, 0, 10)
		for range 10 {
			ids = append(ids, a.Next())
		}
		return ids
	}
	first, second := run(), run()
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("allocation %d differs: %d vs %d", i, first[i], second[i])
		}
	}
}
