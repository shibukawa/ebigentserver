package lan_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/run/lan"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
)

// A two-slot game small enough to encode by hand: each side raises its
// own number, and the match ends when one reaches the target. It has no
// rules worth testing — it exists so the link has something real to
// carry.

const (
	slotHost  session.SlotID = 1
	slotGuest session.SlotID = 2
	target    int32          = 20
	protocol                 = "lan-test-1"
)

type State struct {
	Score  [3]int32
	Ticks  uint32
	Winner uint16
	Over   bool
}

type Action struct{ Raise int32 }

type Observation struct {
	You    session.SlotID
	Score  [3]int32
	Over   bool
	Winner uint16
}

// Delta carries the whole state. Compression is not what this test is
// about, and a codec is free to be honest about that.
type Delta struct{ To State }

type rules struct{}

var _ session.TickSimulation[State, Action, Observation] = rules{}

func (rules) Start(uint64) State                  { return State{} }
func (rules) ActingSlots(*State) []session.SlotID { return []session.SlotID{slotHost, slotGuest} }
func (rules) Apply(s *State, slot session.SlotID, a Action) {
	if s.Over || a.Raise <= 0 {
		return
	}
	s.Score[slot] += a.Raise
}

func (rules) Advance(s *State) {
	if s.Over {
		return
	}
	s.Ticks++
	for _, slot := range []session.SlotID{slotHost, slotGuest} {
		if s.Score[slot] >= target {
			s.Over, s.Winner = true, uint16(slot)
			return
		}
	}
	if s.Ticks > 6000 { // a stuck link must not hang the test
		s.Over = true
	}
}

func (rules) Project(s *State, slot session.SlotID) Observation {
	return Observation{You: slot, Score: s.Score, Over: s.Over, Winner: s.Winner}
}

func (rules) Evaluate(s *State, slot session.SlotID) session.EvaluationSignal {
	sig := session.EvaluationSignal{Score: int64(s.Score[slot])}
	switch {
	case !s.Over:
	case s.Winner == uint16(slot):
		sig.Terminal = session.Win
	case s.Winner == 0:
		sig.Terminal = session.Draw
	default:
		sig.Terminal = session.Lose
	}
	return sig
}

func appendState(dst []byte, s *State) []byte {
	for _, v := range s.Score {
		dst = binary.BigEndian.AppendUint32(dst, uint32(v))
	}
	dst = binary.BigEndian.AppendUint32(dst, s.Ticks)
	dst = binary.BigEndian.AppendUint16(dst, s.Winner)
	if s.Over {
		return append(dst, 1)
	}
	return append(dst, 0)
}

func decodeState(s *State, b []byte) error {
	if len(b) != 3*4+4+2+1 {
		return fmt.Errorf("state: %d bytes", len(b))
	}
	for i := range s.Score {
		s.Score[i] = int32(binary.BigEndian.Uint32(b[i*4:]))
	}
	s.Ticks = binary.BigEndian.Uint32(b[12:])
	s.Winner = binary.BigEndian.Uint16(b[16:])
	s.Over = b[18] == 1
	return nil
}

func codec() statesync.Codec[State, Delta] {
	return statesync.Codec[State, Delta]{
		AppendSnapshot: appendState,
		DecodeSnapshot: decodeState,
		Diff:           func(_, current *State) Delta { return Delta{To: *current} },
		AppendDelta:    func(dst []byte, d *Delta) []byte { return appendState(dst, &d.To) },
		DecodeDelta:    func(d *Delta, b []byte) error { return decodeState(&d.To, b) },
		ApplyDelta:     func(s *State, d Delta) error { *s = d.To; return nil },
	}
}

func encodeInput(dst []byte, a Action) []byte {
	return binary.BigEndian.AppendUint32(dst, uint32(a.Raise))
}

func decodeInput(b []byte) (Action, error) {
	if len(b) != 4 {
		return Action{}, fmt.Errorf("input: %d bytes", len(b))
	}
	return Action{Raise: int32(binary.BigEndian.Uint32(b))}, nil
}

func tuning() session.TuningProfile {
	return session.TuningProfile{
		TickRate: 60, SendRate: 60, HistoryDepth: 8, SnapshotEvery: 30,
	}
}

func options() lan.Options[State, Action, Delta, Observation] {
	return lan.Options[State, Action, Delta, Observation]{
		Name:        "lan-test",
		Protocol:    protocol,
		Codec:       codec(),
		Tuning:      tuning(),
		EncodeInput: encodeInput,
		DecodeInput: decodeInput,
		Project:     rules{}.Project,
	}
}

// raiser is the guest's controller. On this side of the link a person
// and a bot are the same kind of object; this one just always raises.
type raiser struct {
	mu   sync.Mutex
	last Observation
	seen int
}

func (*raiser) Joined(session.SlotID) {}

func (r *raiser) Observe(obs Observation) {
	r.mu.Lock()
	r.last, r.seen = obs, r.seen+1
	r.mu.Unlock()
}

func (r *raiser) Decide(context.Context) (Action, bool) {
	r.mu.Lock()
	over := r.last.Over
	r.mu.Unlock()
	if over {
		return Action{}, false
	}
	return Action{Raise: 1}, true
}

func (*raiser) Ended(session.Result) {}

func (r *raiser) snapshot() (Observation, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last, r.seen
}

// TestTwoInstancesPlayOverTheLink is the whole preset end to end: a host
// gathers a roster, a guest takes the remaining seat over real
// WebSocket, and both play one match to a terminal position.
func TestTwoInstancesPlayOverTheLink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options()
	roster, err := run.NewRoster[State, Action, Observation](
		run.Options{Name: "lan-test", Devices: run.Keyboard, MaxLocalSeats: 1},
		[]session.SlotID{slotHost, slotGuest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roster.JoinLocal("host-player"); err != nil {
		t.Fatal(err)
	}

	host, err := lan.Open(ctx, opts, roster, 7)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	// The guest joins while the lobby is still gathering. Join blocks
	// inside the handshake until the host admits, which is the point of
	// the parking: nobody has to poll.
	guestAgent := &raiser{}
	joined := make(chan *lan.Guest[State, Action, Delta, Observation], 1)
	joinErr := make(chan error, 1)
	go func() {
		g, err := lan.Join(ctx, opts, host.Endpoint())
		if err != nil {
			joinErr <- err
			return
		}
		joined <- g
	}()

	// The lobby sees the arrival as an ordinary seat change.
	deadline := time.Now().Add(5 * time.Second)
	for !roster.Complete() {
		if time.Now().After(deadline) {
			t.Fatalf("guest never reached the roster: %v", roster.Seats())
		}
		select {
		case err := <-joinErr:
			t.Fatalf("join: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	seats := roster.Seats()
	if seats[1].Kind != run.RemoteHuman {
		t.Fatalf("seat 2 kind = %v, want RemoteHuman", seats[1].Kind)
	}

	tune := tuning()
	cfg := session.Config[State, Action, Observation]{
		ID:         "lan-test-0000",
		Slots:      []session.SlotID{slotHost, slotGuest},
		Simulation: rules{},
		Tuning:     &tune,
		Seed:       7,
	}
	host.Attach(&cfg)

	match, err := roster.Finalize(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Serve(ctx, match); err != nil {
		t.Fatal(err)
	}
	match.Start(ctx, session.Paced)

	var guest *lan.Guest[State, Action, Delta, Observation]
	select {
	case guest = <-joined:
	case err := <-joinErr:
		t.Fatalf("join: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("guest was never admitted")
	}
	defer guest.Close()

	if guest.Slot() != slotGuest {
		t.Fatalf("guest seated at %d, want %d", guest.Slot(), slotGuest)
	}
	if guest.Seed() != 7 {
		t.Fatalf("guest seed = %d, want 7 (the handshake carries it)", guest.Seed())
	}

	guestDone := make(chan error, 1)
	go func() { guestDone <- guest.Run(ctx, guestAgent) }()

	// The host's own seat is driven from here, exactly as a window
	// would drive it: submissions into the slot inbox.
	go func() {
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-match.Done():
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				_ = match.Submit(slotHost, Action{Raise: 1})
			}
		}
	}()

	select {
	case <-match.Done():
	case <-ctx.Done():
		t.Fatal("match never finished")
	}
	if err := match.Err(); err != nil {
		t.Fatalf("match: %v", err)
	}

	// Both sides reached a terminal position, and the guest got there
	// from state it reconstructed rather than simulated.
	obs, seen := guestAgent.snapshot()
	if seen == 0 {
		t.Fatal("the guest never received an observation")
	}
	if obs.You != slotGuest {
		t.Fatalf("guest observed itself as slot %d", obs.You)
	}
	if obs.Score[slotGuest] == 0 {
		t.Fatal("the guest's own raises never reached the authoritative state")
	}
	if obs.Score[slotHost] == 0 {
		t.Fatal("the host's raises never reached the guest")
	}
	t.Logf("guest saw host=%d guest=%d over=%v after %d observations",
		obs.Score[slotHost], obs.Score[slotGuest], obs.Over, seen)

	guest.Close()
	if err := <-guestDone; err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("guest loop ended with %v", err)
	}
}

// freeUDPPort picks a loopback port for the beacon. Tests cannot
// broadcast, so both ends are pointed at the same loopback socket
// instead — the code path under test is otherwise identical.
func freeUDPPort(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	_ = pc.Close()
	return addr
}

// TestBrowseFindsTheHost covers the half of the preset that removes the
// address bar: a guest that was told nothing still finds the host, and
// learns where to dial from the beacon alone.
func TestBrowseFindsTheHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	addr := freeUDPPort(t)
	opts := options()
	opts.BeaconAddr, opts.DiscoveryAddr = addr, addr

	roster, err := run.NewRoster[State, Action, Observation](
		run.Options{Name: "lan-test", Devices: run.Keyboard, MaxLocalSeats: 1},
		[]session.SlotID{slotHost, slotGuest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roster.JoinLocal("host-player"); err != nil {
		t.Fatal(err)
	}
	host, err := lan.Open(ctx, opts, roster, 7)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	found, err := lan.Browse(ctx, opts, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("browse found no host")
	}
	b := found[0]
	if b.Endpoint != host.Endpoint() {
		t.Fatalf("beacon endpoint = %q, want %q", b.Endpoint, host.Endpoint())
	}
	if b.ProtocolVersion != protocol {
		t.Fatalf("beacon protocol = %q, want %q", b.ProtocolVersion, protocol)
	}
	if b.PlayerCount != 1 {
		t.Fatalf("beacon reports %d seated, want 1", b.PlayerCount)
	}

	// The endpoint the beacon carried is enough to take a seat.
	joined := make(chan error, 1)
	go func() {
		_, err := lan.Join(ctx, opts, b.Endpoint)
		joined <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !roster.Complete() {
		if time.Now().After(deadline) {
			t.Fatalf("browse-then-join never seated: %v", roster.Seats())
		}
		select {
		case err := <-joined:
			t.Fatalf("join after browse: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestProtocolMismatchIsRefusedAtTheSeat holds the preset to
// rule:protocol-version-must-match at the earliest point it can be
// checked: before a seat is handed out at all.
func TestProtocolMismatchIsRefusedAtTheSeat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	roster, err := run.NewRoster[State, Action, Observation](
		run.Options{Name: "lan-test", Devices: run.Keyboard, MaxLocalSeats: 1},
		[]session.SlotID{slotHost, slotGuest})
	if err != nil {
		t.Fatal(err)
	}
	host, err := lan.Open(ctx, options(), roster, 7)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	stale := options()
	stale.Protocol = "lan-test-0"
	if _, err := lan.Join(ctx, stale, host.Endpoint()); err == nil {
		t.Fatal("a guest speaking an older protocol was seated")
	}
	for _, seat := range roster.Seats() {
		if seat.Filled() {
			t.Fatalf("a refused guest still took seat %d", seat.Slot)
		}
	}
}
