package statesync

import "github.com/shibukawa/ebigentserver/session"

// ViewSender is the per-receiver outbound state pipeline. The plain
// Sender implements it for global visibility (the view IS the world);
// ProjectedSender implements it for scoped visibility, where each
// receiver's stream carries only its projection.
type ViewSender[W any] interface {
	// Send produces the packet for the committed world at tick.
	Send(tick session.Tick, world *W) Packet
	// Confirm records the newest version the receiver is known to hold.
	Confirm(tick session.Tick)
	// ResyncRequested forces the next packet to be a full snapshot.
	ResyncRequested()
}

// ProjectedSender is concept:agent-view: the server-side retained
// per-agent projection, updated incrementally. Each send projects the
// world through the game's visibility predicate into the receiver's view
// type V (rule:sight-content-owned-by-game — the predicate and
// field selection are the game's; retention, diffing, and baselines are
// the framework's, reusing the data:state-delta machinery unchanged).
//
// Because the projection runs before serialization, hidden state is
// never encoded, let alone sent: the scope is a boundary between
// players, not a display filter (concept:visibility-scope's security
// note, policy:sight-scoped-information).
type ProjectedSender[W, V, D any] struct {
	project func(*W) V
	inner   *Sender[V, D]
}

var _ ViewSender[int] = (*ProjectedSender[int, int, int])(nil)

// NewProjectedSender builds the pipeline for one receiver: project the
// world into V, then retain/diff/send V exactly as a world would be.
func NewProjectedSender[W, V, D any](codec Codec[V, D], tuning session.TuningProfile, project func(*W) V) (*ProjectedSender[W, V, D], error) {
	if project == nil {
		return nil, errValidate("statesync: project function is required")
	}
	inner, err := NewSender(codec, tuning)
	if err != nil {
		return nil, err
	}
	return &ProjectedSender[W, V, D]{project: project, inner: inner}, nil
}

// Send projects and delegates.
func (p *ProjectedSender[W, V, D]) Send(tick session.Tick, world *W) Packet {
	view := p.project(world)
	return p.inner.Send(tick, &view)
}

// Confirm delegates.
func (p *ProjectedSender[W, V, D]) Confirm(tick session.Tick) { p.inner.Confirm(tick) }

// ResyncRequested delegates.
func (p *ProjectedSender[W, V, D]) ResyncRequested() { p.inner.ResyncRequested() }

type errValidate string

func (e errValidate) Error() string { return string(e) }
