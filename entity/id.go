// Package entity implements decision:owner-namespaced-entity-ids: entity
// identifiers namespaced by their creating authority, so allocation never
// needs coordination and rollback resimulation reproduces identical IDs.
package entity

// ID is one entity identifier. The owner namespace (a session.SlotID, or 0
// for entities the session itself spawns) packs into the high 16 bits and
// a per-owner counter into the low 48, so two creators can never collide
// by construction and the wire form stays a small integer
// (concept:cbor-wire-profile).
type ID uint64

const seqBits = 48

// Compose builds an ID from an owner namespace and a sequence number. The
// sequence is masked to 48 bits; a session would need 2^48 entities from
// one owner to wrap, which is treated as impossible rather than checked
// per allocation.
func Compose(owner uint16, seq uint64) ID {
	return ID(uint64(owner)<<seqBits | seq&(1<<seqBits-1))
}

// Owner returns the creating authority's namespace.
func (id ID) Owner() uint16 { return uint16(id >> seqBits) }

// Seq returns the per-owner sequence number.
func (id ID) Seq() uint64 { return uint64(id) & (1<<seqBits - 1) }

// Allocator hands out IDs for one owner. It must advance only inside
// simulation steps: the counter is part of the world state, which is what
// makes replay regenerate identical IDs (actor:replay-agent depends on
// this). The zero value allocates for the session namespace.
type Allocator struct {
	// OwnerID is the namespace, a slot ID or 0 for the session.
	OwnerID uint16
	// NextSeq is the next sequence number to hand out. Exported so the
	// allocator serializes with the world state it belongs to.
	NextSeq uint64
}

// Next returns the next ID and advances the counter.
func (a *Allocator) Next() ID {
	id := Compose(a.OwnerID, a.NextSeq)
	a.NextSeq++
	return id
}
