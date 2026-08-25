package run

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
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
type Roster[W, A, S any] struct {
	opts  Options
	slots []session.SlotID

	mu       sync.Mutex
	seats    map[session.SlotID]Seat
	agents   map[session.SlotID]session.Agent[S, A]
	watchers []func([]Seat)
}

// NewRoster declares the seat set. slots is the game's own slot set, so
// the roster can never invent a seat the rules did not declare.
func NewRoster[W, A, S any](opts Options, slots []session.SlotID) (*Roster[W, A, S], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if len(slots) == 0 {
		return nil, fmt.Errorf("run: %s declares no slots", opts.Name)
	}
	sorted := slices.Clone(slots)
	slices.Sort(sorted)
	r := &Roster[W, A, S]{
		opts:   opts,
		slots:  sorted,
		seats:  make(map[session.SlotID]Seat, len(sorted)),
		agents: make(map[session.SlotID]session.Agent[S, A], len(sorted)),
	}
	for _, slot := range sorted {
		r.seats[slot] = Seat{Slot: slot}
	}
	return r, nil
}

// Slots reports the declared slot set in commit order.
func (r *Roster[W, A, S]) Slots() []session.SlotID { return slices.Clone(r.slots) }

// Seats reports the current roster in slot order. The result is a copy:
// a lobby may hold it across frames.
func (r *Roster[W, A, S]) Seats() []Seat {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshot()
}

// snapshot builds the seat list. Callers hold the lock.
func (r *Roster[W, A, S]) snapshot() []Seat {
	out := make([]Seat, 0, len(r.slots))
	for _, slot := range r.slots {
		out = append(out, r.seats[slot])
	}
	return out
}

// OnChange registers a callback fired after every roster change, with the
// new seat list. ui:lobby-scene renders from it; a game's own scene may
// use it instead. Callbacks run on the goroutine that made the change, so
// a callback that blocks stalls whoever guest.
func (r *Roster[W, A, S]) OnChange(f func([]Seat)) {
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
func (r *Roster[W, A, S]) notify(seats []Seat) {
	r.mu.Lock()
	watchers := slices.Clone(r.watchers)
	r.mu.Unlock()
	for _, f := range watchers {
		f(seats)
	}
}

// Sit claims a specific slot. It is the raw seating call every other one
// is written in terms of, and what a game's own gathering screen uses
// when it wants to decide the arrangement itself.
//
// Sitting is one act wherever the occupant is. A person at this keyboard
// and a person across a link take a seat the same way, and which of the
// two it was is reported on the seat rather than being a different verb.
//
// agent is the controller for a bot seat. A human seat passes nil: its
// actions arrive through the slot's inbox, which is the same path a
// remote peer uses, so the session sees session.Detached either way.
// local says the occupant decides inside this process. It is the one
// thing a caller knows that the roster cannot work out for itself, and it
// is reported on the seat rather than folded into the kind.
func (r *Roster[W, A, S]) Sit(slot session.SlotID, kind SeatKind, local bool, id string, agent session.Agent[S, A]) error {
	if kind == Empty {
		return fmt.Errorf("run: cannot sit slot %d as %v", slot, kind)
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
func (r *Roster[W, A, S]) localHumans() int {
	n := 0
	for _, seat := range r.seats {
		if seat.LocalHuman() {
			n++
		}
	}
	return n
}

// SitLocal seats a person at this machine in the lowest free slot — what
// a start button, a click, or a key press does in ui:lobby-scene.
func (r *Roster[W, A, S]) SitLocal(id string) (session.SlotID, error) {
	return r.joinFree(Human, true, id, nil)
}

// SitRemote seats a person arriving over a link. flow:session-admission
// calls it after the ticket verifies; no screen is involved.
func (r *Roster[W, A, S]) SitRemote(slot session.SlotID, id string) error {
	return r.Sit(slot, Human, false, id, nil)
}

// SitBot seats a controller in the lowest free slot. The enemies of a
// solo game, the opponent of a practice match, and a seat a departed
// player left behind are all this call (concept:agent-proxy-designation:
// takeover needs no mechanism beyond seating an agent).
func (r *Roster[W, A, S]) SitBot(id string, agent session.Agent[S, A]) (session.SlotID, error) {
	return r.joinFree(Bot, true, id, agent)
}

// joinFree claims the lowest free slot.
func (r *Roster[W, A, S]) joinFree(kind SeatKind, local bool, id string, agent session.Agent[S, A]) (session.SlotID, error) {
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
	if err := r.Sit(target, kind, local, id, agent); err != nil {
		return 0, err
	}
	return target, nil
}

// Leave empties a seat. Before play only: departure during a match is
// concept:agent-departure-policy and belongs to the session.
func (r *Roster[W, A, S]) Leave(slot session.SlotID) {
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
func (r *Roster[W, A, S]) SetReady(slot session.SlotID, ready bool) {
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
func (r *Roster[W, A, S]) AgentKinds() map[session.SlotID]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[session.SlotID]string, len(r.seats))
	for slot, seat := range r.seats {
		out[slot] = seat.AgentKind()
	}
	return out
}

// Complete reports whether every declared slot is filled.
func (r *Roster[W, A, S]) Complete() bool {
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
func (r *Roster[W, A, S]) Ready() bool {
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
// FillNamed seats the controllers a run asked for by name.
//
// It runs before FillBots and fills only the slots the assignment
// mentions, which is what keeps "record the chaser" from meaning "make
// every seat a chaser". A seat somebody already took is left alone: an
// assignment seeds a roster (api:roster) rather than overruling it.
//
// An unknown name is an error rather than a fallback. A run asking for
// an agent this game does not have would otherwise record a corpus under
// a label nothing in it earned, and that is worse than not running.
func (r *Roster[W, A, S]) FillNamed(agents map[string]func(seed uint64) session.Agent[S, A], want map[session.SlotID]string, seed uint64) error {
	if len(want) == 0 {
		return nil
	}
	for _, slot := range r.Slots() {
		name, asked := want[slot]
		if !asked {
			// A run that named no seat in particular named all of
			// them, which is what recording one kind at a time means.
			name, asked = want[AnySlot]
		}
		if !asked {
			continue
		}
		build, ok := agents[name]
		if !ok {
			return fmt.Errorf("run: slot %d asks for agent %q, which this game does not declare; Binding.Agents names %s",
				slot, name, declared(agents))
		}
		r.mu.Lock()
		filled := r.seats[slot].Filled()
		r.mu.Unlock()
		if filled {
			continue
		}
		if err := r.Sit(slot, Bot, true, name, build(seed)); err != nil {
			return err
		}
	}
	return nil
}

// declared lists what a binding offers, for the error that says a run
// asked for something else.
func declared[S, A any](agents map[string]func(seed uint64) session.Agent[S, A]) string {
	if len(agents) == 0 {
		return "none"
	}
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func (r *Roster[W, A, S]) FillBots(newAgent func(slot session.SlotID) (id string, agent session.Agent[S, A])) error {
	// The factory is only needed for a seat that is still empty. A run
	// that named a controller for every seat has already answered the
	// question NewAgent exists to answer, and demanding one anyway would
	// make Binding.Agents unusable on its own.
	for _, slot := range r.Slots() {
		r.mu.Lock()
		filled := r.seats[slot].Filled()
		r.mu.Unlock()
		if filled {
			continue
		}
		if newAgent == nil {
			return fmt.Errorf("run: slot %d is empty and there is no Binding.NewAgent to fill it; "+
				"declare one, name an agent for it, or leave the seat for somebody arriving over a link", slot)
		}
		id, agent := newAgent(slot)
		if err := r.Sit(slot, Bot, true, id, agent); err != nil {
			return err
		}
	}
	for _, slot := range r.Slots() {
		r.SetReady(slot, true)
	}
	return nil
}

// AnySlot is the assignment key that stands for every bot seat.
//
// Slot 0 belongs to the session and is never a player slot
// (decision:owner-namespaced-entity-ids), so it is free to mean "any" —
// and a run recording one enemy kind wants exactly that: this
// controller wherever a bot would otherwise be chosen.
const AnySlot session.SlotID = 0

// ParseAgents reads a seat assignment as one value.
//
// It is one value rather than a table because that is what can travel:
// an array of tables in data:run-config has the file as its only source,
// so a tool passing an assignment down to a child process has nowhere to
// put one. The slot table of api:roster is the larger thing — seat
// identity, human or bot, teams — and this is not it.
//
//	chaser              every bot seat plays the chaser
//	2=chaser,3=flanker  those two seats do, the rest are the game's choice
//	chaser,1=runner     a default with one seat named against it
func ParseAgents(spec string) (map[session.SlotID]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	out := map[session.SlotID]string{}
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		slot, name, keyed := strings.Cut(field, "=")
		slot, name = strings.TrimSpace(slot), strings.TrimSpace(name)
		if !keyed {
			slot, name = "0", slot
		}
		n, err := strconv.Atoi(slot)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("run: %q is not a seat assignment; write a name, or slot=name", field)
		}
		if name == "" {
			return nil, fmt.Errorf("run: seat %s names no agent", slot)
		}
		out[session.SlotID(n)] = name
	}
	return out, nil
}
