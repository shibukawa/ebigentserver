package run

import (
	"fmt"
	"slices"
	"sync"

	"github.com/shibukawa/ebigentserver/session"
)

// Roster is api:roster: the mutable list of who will play, filled during
// the gathering state of concept:match-lifecycle and frozen into a
// concept:session by Finalize.
//
// Two kinds of caller write to it and neither knows about the other: a
// screen (ui:lobby-scene or a game's own title scene) and a link
// (flow:session-admission). It is therefore safe for concurrent use.
//
// A solo game uses it too. Its enemies are seats like any other, filled
// by ordinary agents, which is what puts every enemy decision into
// data:episode-log without the game arranging anything.
type Roster[S, A, O any] struct {
	opts  Options
	slots []session.SlotID

	mu       sync.Mutex
	seats    map[session.SlotID]Seat
	agents   map[session.SlotID]session.Agent[O, A]
	watchers []func([]Seat)
}

// NewRoster declares the seat set. slots is the game's own slot set, so
// the roster can never invent a seat the rules did not declare.
func NewRoster[S, A, O any](opts Options, slots []session.SlotID) (*Roster[S, A, O], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if len(slots) == 0 {
		return nil, fmt.Errorf("run: %s declares no slots", opts.Name)
	}
	sorted := slices.Clone(slots)
	slices.Sort(sorted)
	r := &Roster[S, A, O]{
		opts:   opts,
		slots:  sorted,
		seats:  make(map[session.SlotID]Seat, len(sorted)),
		agents: make(map[session.SlotID]session.Agent[O, A], len(sorted)),
	}
	for _, slot := range sorted {
		r.seats[slot] = Seat{Slot: slot}
	}
	return r, nil
}

// Slots reports the declared slot set in commit order.
func (r *Roster[S, A, O]) Slots() []session.SlotID { return slices.Clone(r.slots) }

// Seats reports the current roster in slot order. The result is a copy:
// a lobby may hold it across frames.
func (r *Roster[S, A, O]) Seats() []Seat {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshot()
}

// snapshot builds the seat list. Callers hold the lock.
func (r *Roster[S, A, O]) snapshot() []Seat {
	out := make([]Seat, 0, len(r.slots))
	for _, slot := range r.slots {
		out = append(out, r.seats[slot])
	}
	return out
}

// OnChange registers a callback fired after every roster change, with the
// new seat list. ui:lobby-scene renders from it; a game's own scene may
// use it instead. Callbacks run on the goroutine that made the change, so
// a callback that blocks stalls whoever joined.
func (r *Roster[S, A, O]) OnChange(f func([]Seat)) {
	if f == nil {
		return
	}
	r.mu.Lock()
	r.watchers = append(r.watchers, f)
	seats := r.snapshot()
	r.mu.Unlock()
	f(seats)
}

// notify fires the watchers. Callers must not hold the lock.
func (r *Roster[S, A, O]) notify(seats []Seat) {
	r.mu.Lock()
	watchers := slices.Clone(r.watchers)
	r.mu.Unlock()
	for _, f := range watchers {
		f(seats)
	}
}

// Take claims a specific slot. It is the raw seating call every other one
// is written in terms of, and what a game's own scene uses when it wants
// to decide the arrangement itself.
//
// agent is the controller for a bot seat. A human seat passes nil: its
// actions arrive through the slot's inbox, which is the same path a
// remote peer uses, so the session sees session.Detached either way.
// local says the occupant decides inside this process. It is the one
// thing a caller knows that the roster cannot work out for itself, and it
// is reported on the seat rather than folded into the kind.
func (r *Roster[S, A, O]) Take(slot session.SlotID, kind SeatKind, local bool, id string, agent session.Agent[O, A]) error {
	if kind == Empty {
		return fmt.Errorf("run: cannot take slot %d as %v", slot, kind)
	}
	if kind == Bot && local && agent == nil {
		return fmt.Errorf("run: local %v seat %d needs an agent", kind, slot)
	}
	r.mu.Lock()
	seat, ok := r.seats[slot]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("%w: %d", ErrUnknownSlot, slot)
	}
	if seat.Filled() {
		r.mu.Unlock()
		return fmt.Errorf("%w: %d held by %v", ErrSeatTaken, slot, seat.Kind)
	}
	if kind == Human && local && r.localHumans() >= r.opts.localSeatLimit() {
		r.mu.Unlock()
		return fmt.Errorf("%w: %d local seats", ErrLocalSeatLimit, r.opts.localSeatLimit())
	}
	r.seats[slot] = Seat{Slot: slot, Kind: kind, Local: local, ID: id}
	if agent != nil {
		r.agents[slot] = agent
	}
	seats := r.snapshot()
	r.mu.Unlock()
	r.notify(seats)
	return nil
}

// localHumans counts people sitting at this machine. Callers hold the
// lock.
//
// Local bots are not counted. MaxLocalSeats bounds how many people share
// one screen, which is a fact about the screen; a solo game may seat any
// number of enemy agents in the same process without touching it.
func (r *Roster[S, A, O]) localHumans() int {
	n := 0
	for _, seat := range r.seats {
		if seat.LocalHuman() {
			n++
		}
	}
	return n
}

// JoinLocal seats a person at this machine in the lowest free slot — what
// a start button, a click, or a key press does in ui:lobby-scene.
func (r *Roster[S, A, O]) JoinLocal(id string) (session.SlotID, error) {
	return r.joinFree(Human, true, id, nil)
}

// JoinRemote seats a person arriving over a link. flow:session-admission
// calls it after the ticket verifies; no screen is involved.
func (r *Roster[S, A, O]) JoinRemote(slot session.SlotID, id string) error {
	return r.Take(slot, Human, false, id, nil)
}

// AddBot seats a controller in the lowest free slot. The enemies of a
// solo game, the opponent of a practice match, and a seat a departed
// player left behind are all this call (concept:agent-proxy-designation:
// takeover needs no mechanism beyond seating an agent).
func (r *Roster[S, A, O]) AddBot(id string, agent session.Agent[O, A]) (session.SlotID, error) {
	return r.joinFree(Bot, true, id, agent)
}

// joinFree claims the lowest free slot.
func (r *Roster[S, A, O]) joinFree(kind SeatKind, local bool, id string, agent session.Agent[O, A]) (session.SlotID, error) {
	r.mu.Lock()
	var target session.SlotID
	found := false
	for _, slot := range r.slots {
		if !r.seats[slot].Filled() {
			target, found = slot, true
			break
		}
	}
	r.mu.Unlock()
	if !found {
		return 0, ErrNoFreeSeat
	}
	if err := r.Take(target, kind, local, id, agent); err != nil {
		return 0, err
	}
	return target, nil
}

// Leave empties a seat. Before play only: departure during a match is
// concept:agent-departure-policy and belongs to the session.
func (r *Roster[S, A, O]) Leave(slot session.SlotID) {
	r.mu.Lock()
	if _, ok := r.seats[slot]; !ok {
		r.mu.Unlock()
		return
	}
	r.seats[slot] = Seat{Slot: slot}
	delete(r.agents, slot)
	seats := r.snapshot()
	r.mu.Unlock()
	r.notify(seats)
}

// SetReady marks a seat ready to start.
func (r *Roster[S, A, O]) SetReady(slot session.SlotID, ready bool) {
	r.mu.Lock()
	seat, ok := r.seats[slot]
	if !ok || !seat.Filled() || seat.Ready == ready {
		r.mu.Unlock()
		return
	}
	seat.Ready = ready
	r.seats[slot] = seat
	seats := r.snapshot()
	r.mu.Unlock()
	r.notify(seats)
}

// AgentKinds labels every seat for the data:episode-log header, so a
// corpus can be filtered by who decided — the axis analysis and Phase 7
// distillation actually select on.
func (r *Roster[S, A, O]) AgentKinds() map[session.SlotID]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[session.SlotID]string, len(r.seats))
	for slot, seat := range r.seats {
		out[slot] = seat.AgentKind()
	}
	return out
}

// Complete reports whether every declared slot is filled.
func (r *Roster[S, A, O]) Complete() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, seat := range r.seats {
		if !seat.Filled() {
			return false
		}
	}
	return true
}

// Ready reports whether every seat is filled and has confirmed. It is the
// gathering-to-running condition of concept:match-lifecycle, and a lobby
// that has no ready step simply marks seats ready as they join.
func (r *Roster[S, A, O]) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, seat := range r.seats {
		if !seat.Filled() || !seat.Ready {
			return false
		}
	}
	return true
}

// FillBots seats an agent in every remaining slot and marks the roster
// ready. It is what a solo game does after the player takes their seat,
// and what a headless training run does for every seat including the
// player's, so the same rules produce a corpus with nobody watching.
func (r *Roster[S, A, O]) FillBots(newAgent func(slot session.SlotID) (id string, agent session.Agent[O, A])) error {
	if newAgent == nil {
		return fmt.Errorf("run: no Binding.NewAgent, so empty seats cannot be filled with bots; " +
			"declare one, or leave the seats for people arriving over a link")
	}
	for _, slot := range r.Slots() {
		r.mu.Lock()
		filled := r.seats[slot].Filled()
		r.mu.Unlock()
		if filled {
			continue
		}
		id, agent := newAgent(slot)
		if err := r.Take(slot, Bot, true, id, agent); err != nil {
			return err
		}
	}
	for _, slot := range r.Slots() {
		r.SetReady(slot, true)
	}
	return nil
}
