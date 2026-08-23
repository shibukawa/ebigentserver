//go:build !js && !wasip1

// Package lan is the LAN preset: two instances of one binary find each
// other on the same network and play, with no server to run and no
// address to type.
//
// It exists because the pieces underneath are each small and together
// are not — a beacon (api:lan-discovery), a ticket
// (flow:session-admission), a listener, api:sequence-ack-layer, and
// concept:state-synchronization. A game that wants a person to be able
// to join declares its wire types once and calls Open or Join.
//
// Network scope is the control here
// (rule:unauthenticated-admission-requires-scope-or-capability): the
// host refuses to listen anywhere but loopback, a private range, or
// link-local, and mints short-lived tickets from a key it generates at
// startup. There is no account, no broker, and nothing shared outside
// the segment.
//
// Ordering is the one thing a caller has to respect, and it comes from
// the session rather than from here. A peer can only be admitted once
// its seat has an inbox, which means once the session exists — but
// people join while the lobby is still gathering, before there is one.
// So a guest's connection is accepted early and parked, and Serve
// admits every parked connection the moment the match begins.
package lan

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/budget"
	"github.com/shibukawa/ebigentserver/discovery"
	"github.com/shibukawa/ebigentserver/netplay"
	"github.com/shibukawa/ebigentserver/observe"
	"github.com/shibukawa/ebigentserver/run"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/ws"
)

// ticketLife is how long a lobby ticket stays redeemable. It only has to
// outlive the walk from the browse list to the start press.
const ticketLife = 2 * time.Minute

// roomRefresh is how often a guest that is only looking re-reads the
// roster. Somebody arriving should show up while the player is still
// deciding, and a second is faster than anyone reads a list.
const roomRefresh = time.Second

// Options declares what a game puts on the wire. Every field but the
// test overrides is required: this package encodes nothing on its own,
// it only carries what the game's generated codec produces.
type Options[W, A, D, S any] struct {
	// Name is what the browse list shows.
	Name string
	// Protocol is data:protocol-version, compared before anything else
	// (rule:protocol-version-must-match).
	Protocol string
	// Codec is the game's generated snapshot and delta encoding.
	Codec statesync.Codec[W, D]
	// Tuning is the declared data:session-tuning-profile.
	Tuning session.TuningProfile
	// Budget bounds connections and input rate. The zero value takes
	// the package default, which suits a handful of players.
	Budget budget.Budget
	// EncodeInput and DecodeInput carry data:player-input.
	EncodeInput func(dst []byte, a A) []byte
	DecodeInput func(b []byte) (A, error)
	// Project builds a slot's sight on the guest side, from the
	// world the guest reconstructed.
	Project func(world *W, slot session.SlotID) S
	// Port is the host's listening port; 0 picks a free one.
	Port int

	// BrowseWindow is how long Discover listens before answering. Zero
	// takes a second and a half, which is two beacon intervals plus
	// slack — long enough that a host that just started is seen, short
	// enough that a player alone on the network is not left staring.
	BrowseWindow time.Duration

	// BeaconAddr overrides the broadcast destination and DiscoveryAddr
	// the listen address. Both are for tests, which cannot broadcast.
	BeaconAddr    string
	DiscoveryAddr string
}

func (o Options[W, A, D, S]) browseWindow() time.Duration {
	if o.BrowseWindow > 0 {
		return o.BrowseWindow
	}
	return 1500 * time.Millisecond
}

func (o Options[W, A, D, S]) validate() error {
	var missing []string
	if o.Protocol == "" {
		missing = append(missing, "Protocol")
	}
	if o.EncodeInput == nil {
		missing = append(missing, "EncodeInput")
	}
	if o.DecodeInput == nil {
		missing = append(missing, "DecodeInput")
	}
	if o.Project == nil {
		missing = append(missing, "Project")
	}
	if o.Codec.AppendSnapshot == nil || o.Codec.DecodeSnapshot == nil {
		missing = append(missing, "Codec")
	}
	if len(missing) > 0 {
		return fmt.Errorf("lan: %v required", missing)
	}
	return o.Tuning.Validate()
}

func (o Options[W, A, D, S]) budgetOrDefault() budget.Budget {
	b := o.Budget
	if b.MaxConnections == 0 {
		b = budget.Budget{
			MaxSessions: 1, MaxConnections: 8, MaxAgents: 8,
			MaxPendingActions: 64, AdmissionPerSecond: 8,
			InputsPerTick: 4, InputBytesPerSecond: 64 << 10,
			SendQueueBytes: 1 << 20, MaxMessageSize: 64 << 10,
			MaxPendingReassembly: 8, DrainDeadlineMillis: 1000,
		}
	}
	return b
}

// grant is what /join answers: which seat the caller may take and the
// ticket that proves it.
type grant struct {
	Ticket string `json:"ticket"`
	Seat   uint16 `json:"seat"`
}

// Host is one instance offering its match to the network.
type Host[W, A, D, S any] struct {
	opts   Options[W, A, D, S]
	roster *run.Roster[W, A, S] // guarded by mu

	endpoint string
	priv     ed25519.PrivateKey
	verifier *admission.Verifier
	seed     uint64

	listener net.Listener
	http     *http.Server
	upgrader *ws.Upgrader

	mu        sync.Mutex
	parked    []transport.Conn
	admitted  []transport.Conn
	closed    bool
	reportErr func(error)

	server atomic.Pointer[netplay.Server[W, A]]

	stop  context.CancelFunc
	wg    sync.WaitGroup
	peers sync.WaitGroup
}

// Open starts listening and announcing. The roster is the one the lobby
// is filling: a guest reaching /join takes a seat in it, so the lobby
// shows the arrival without knowing a network exists.
func Open[W, A, D, S any](ctx context.Context, opts Options[W, A, D, S], roster *run.Roster[W, A, S], seed uint64) (*Host[W, A, D, S], error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	if roster == nil {
		return nil, errors.New("lan: Open needs the roster the lobby is filling")
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(opts.Port))
	if err != nil {
		return nil, fmt.Errorf("lan: listen: %w", err)
	}
	host, err := advertisedHost()
	if err != nil {
		ln.Close()
		return nil, err
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	endpoint := net.JoinHostPort(host, port)
	// The segment is the only thing keeping strangers out, so refuse to
	// offer this anywhere the segment is not a boundary.
	if err := admission.GuardUnauthenticated(endpoint); err != nil {
		ln.Close()
		return nil, err
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		ln.Close()
		return nil, err
	}
	h := &Host[W, A, D, S]{
		opts:     opts,
		roster:   roster,
		endpoint: endpoint,
		priv:     priv,
		seed:     seed,
		listener: ln,
		upgrader: ws.NewUpgrader(),
		verifier: &admission.Verifier{
			Keys:     map[string]ed25519.PublicKey{"lan": pub},
			Audience: endpoint,
			Leeway:   30 * time.Second,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/room", h.handleRoom)
	mux.HandleFunc("/sit", h.handleSit)
	mux.HandleFunc("/ws", h.handleWS)
	h.http = &http.Server{Handler: mux}

	runCtx, cancel := context.WithCancel(ctx)
	h.stop = cancel
	h.wg.Add(2)
	go func() { defer h.wg.Done(); _ = h.http.Serve(ln) }()
	go func() { defer h.wg.Done(); h.announce(runCtx) }()
	return h, nil
}

// Endpoint is the host:port a guest dials, and the audience its tickets
// name.
func (h *Host[W, A, D, S]) Endpoint() string { return h.endpoint }

// Rebind points the offer at the next match's roster. A roster belongs
// to one match, so without this the offer would keep seating arrivals
// into the finished one, and the new lobby would wait for people who had
// already arrived somewhere else.
func (h *Host[W, A, D, S]) Rebind(r *run.Roster[W, A, S]) {
	h.mu.Lock()
	h.roster = r
	h.mu.Unlock()
	h.server.Store(nil)
	h.dropAdmitted()
}

// currentRoster reads the roster the offer is filling.
func (h *Host[W, A, D, S]) currentRoster() *run.Roster[W, A, S] {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.roster
}

// announce repeats the beacon until the context ends.
func (h *Host[W, A, D, S]) announce(ctx context.Context) {
	a := &discovery.Announcer{
		Addr: h.opts.BeaconAddr,
		Beacon: func() discovery.Beacon {
			seated := 0
			for _, s := range h.currentRoster().Seats() {
				if s.Filled() {
					seated++
				}
			}
			return discovery.Beacon{
				Session:         h.opts.Name,
				Endpoint:        h.endpoint,
				ProtocolVersion: h.opts.Protocol,
				PlayerCount:     seated,
				TicketRequired:  true,
			}
		},
	}
	_ = a.Run(ctx)
}

// handleRoom publishes what the room looks like from outside: its name
// and its seats. It grants nothing and costs the room nothing, which is
// what lets somebody read the roster and leave again.
func (h *Host[W, A, D, S]) handleRoom(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("protocol") != h.opts.Protocol {
		http.Error(w, "protocol mismatch: host speaks "+h.opts.Protocol, http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(run.Room{
		Title: h.opts.Name,
		Seats: h.currentRoster().Seats(),
	})
}

// handleSit seats the caller in the roster and mints the ticket naming
// that seat. This is the whole of the control plane, and it lives inside
// the host process because on a segment there is nothing else to ask.
//
// Nobody is judged here. The room stated its terms when it opened, and
// what happens now is a check against them: the version has to match and
// a seat has to be free. A turnstile rather than a doorman.
func (h *Host[W, A, D, S]) handleSit(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("protocol") != h.opts.Protocol {
		http.Error(w, "protocol mismatch: host speaks "+h.opts.Protocol, http.StatusConflict)
		return
	}
	roster := h.currentRoster()
	slot, err := h.claimFreeSeat(roster, r.RemoteAddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	now := time.Now()
	ticket, err := admission.Sign(h.priv, "lan", admission.Claims{
		Subject:   r.RemoteAddr,
		Audience:  h.endpoint,
		ExpiresAt: now.Add(ticketLife).Unix(),
		ID:        fmt.Sprintf("%s-%d-%d", h.opts.Name, slot, now.UnixNano()),
		SessionID: h.opts.Name,
		Seat:      uint16(slot),
		Role:      "player",
	})
	if err != nil {
		roster.Leave(slot)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(grant{Ticket: ticket, Seat: uint16(slot)})
}

// claimFreeSeat takes the lowest seat nobody holds. Two guests arriving
// together race here, and the roster's own lock decides.
func (h *Host[W, A, D, S]) claimFreeSeat(roster *run.Roster[W, A, S], id string) (session.SlotID, error) {
	for _, seat := range roster.Seats() {
		if seat.Filled() {
			continue
		}
		if err := roster.SitRemote(seat.Slot, id); err == nil {
			return seat.Slot, nil
		}
	}
	return 0, errors.New("lan: every seat is taken")
}

// handleWS parks an upgraded connection. It is not admitted yet: the
// session it would be admitted into does not exist until the lobby
// starts the match.
func (h *Host[W, A, D, S]) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r)
	if err != nil {
		return
	}
	if sv := h.server.Load(); sv != nil {
		// The match is already running; admit straight away.
		go h.admit(context.Background(), sv, conn)
		return
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	h.parked = append(h.parked, conn)
	h.mu.Unlock()
}

// Attach installs the downstream hook before the session is built. The
// netplay server does not exist yet, so the hook reads it each tick and
// does nothing until Serve puts one there.
func (h *Host[W, A, D, S]) Attach(cfg *session.Config[W, A, S]) {
	prev := cfg.Broadcast
	cfg.Broadcast = func(tick session.Tick, world *W) {
		if prev != nil {
			prev(tick, world)
		}
		if sv := h.server.Load(); sv != nil {
			sv.Broadcast(tick, world)
		}
	}
}

// Serve wires the finalized match to the network and admits everyone who
// was waiting. Call it immediately after Finalize and before the first
// tick commits, so no state is produced with nobody to send it to.
func (h *Host[W, A, D, S]) Serve(ctx context.Context, m *run.Match[W, A, S]) error {
	if m == nil {
		return errors.New("lan: Serve needs the finalized match")
	}
	b := h.opts.budgetOrDefault()
	sv, err := netplay.NewServer(ctx, netplay.ServerConfig[W, A]{
		SessionID: h.opts.Name,
		Protocol:  h.opts.Protocol,
		Verifier:  h.verifier,
		Seed:      h.seed,
		Tuning:    h.opts.Tuning,
		Budget:    b,
		MakeSender: func(session.SlotID, string) (statesync.ViewSender[W], error) {
			return statesync.NewSender(h.opts.Codec, h.opts.Tuning)
		},
		DecodeInput: h.opts.DecodeInput,
		Inbox:       m.Inbox,
		Metrics:     &observe.Metrics{},
		Events:      observe.NewLog(64),
	})
	if err != nil {
		return err
	}
	h.server.Store(sv)

	h.mu.Lock()
	waiting := h.parked
	h.parked = nil
	h.mu.Unlock()
	for _, conn := range waiting {
		go h.admit(ctx, sv, conn)
	}
	go h.endWith(m)
	return nil
}

// admit completes one handshake and starts the peer's own loop, which is
// what carries data:player-input the other way. Without it a guest is
// seen and sent to, and never heard.
//
// It runs on its own goroutine because the handshake waits on the other
// end, and the match must not: a guest that never says hello is a seat
// that stays silent, not a match that never starts.
func (h *Host[W, A, D, S]) admit(ctx context.Context, sv *netplay.Server[W, A], conn transport.Conn) {
	peer, err := sv.Admit(ctx, conn)
	if err != nil {
		_ = conn.Close()
		if f := h.onError(); f != nil {
			f(err)
		}
		return
	}
	h.mu.Lock()
	h.admitted = append(h.admitted, conn)
	h.mu.Unlock()
	h.peers.Add(1)
	go peer.Run(ctx, &h.peers)
}

// endWith closes every admitted link once the match is over.
//
// Without this a guest waits forever on a link that will never speak
// again: the session stopped broadcasting, but nothing told the other
// end, so its receive loop sits on a socket that is still open. The
// match ending is the host's news to deliver, and closing the link is
// how it is delivered (decision:host-loss-ends-session — the far side
// reports and returns rather than trying to carry on).
func (h *Host[W, A, D, S]) endWith(m *run.Match[W, A, S]) {
	<-m.Done()
	h.dropAdmitted()
}

// dropAdmitted closes and forgets the links of a finished match.
func (h *Host[W, A, D, S]) dropAdmitted() {
	h.mu.Lock()
	conns := h.admitted
	h.admitted = nil
	h.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

// onError reports the sink for admission failures, if the game set one.
func (h *Host[W, A, D, S]) onError() func(error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.reportErr
}

// OnError registers a sink for admission failures — a guest whose
// protocol drifted, or one that went away between taking a seat and
// saying hello. Without it they are silent, which is the right default
// for a game: one guest failing is not the match failing.
func (h *Host[W, A, D, S]) OnError(f func(error)) {
	h.mu.Lock()
	h.reportErr = f
	h.mu.Unlock()
}

// Close stops announcing and listening. Connections already admitted
// belong to the match and end with it.
func (h *Host[W, A, D, S]) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	waiting := append(h.parked, h.admitted...)
	h.parked, h.admitted = nil, nil
	h.mu.Unlock()
	for _, c := range waiting {
		_ = c.Close()
	}
	h.stop()
	err := h.http.Close()
	h.wg.Wait()
	h.peers.Wait()
	return err
}

// Browse listens for beacons for the given window and reports what
// answered, newest occupancy first seen. A browse is passive: it sends
// nothing, so it cannot be used to find hosts outside the segment.
func Browse[W, A, D, S any](ctx context.Context, opts Options[W, A, D, S], window time.Duration) ([]discovery.Beacon, error) {
	l, err := discovery.Listen(opts.DiscoveryAddr, opts.Protocol)
	if err != nil {
		return nil, err
	}
	defer l.Close()
	deadline := time.NewTimer(window)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return l.Sessions(), ctx.Err()
		case <-deadline.C:
			return l.Sessions(), nil
		case <-tick.C:
			if found := l.Sessions(); len(found) > 0 {
				return found, nil
			}
		}
	}
}

// Guest is this instance playing somebody else's match. It holds no
// session: the world arrives already committed, and the only thing sent
// back is data:player-input.
type Guest[W, A, D, S any] struct {
	opts     Options[W, A, D, S]
	endpoint string
	box      mailbox[A]

	ready chan struct{}

	mu      sync.Mutex
	room    run.Room
	conn    transport.Conn
	ticket  string
	seat    session.SlotID
	seated  bool
	client  *netplay.Client[W, A, D, S]
	onWorld func(session.Tick, *W)
	over    bool
	stop    context.CancelFunc
}

// MatchAt reaches a host and reads its room. It takes no seat: a player
// is entitled to see who is already there and leave, and leaving from
// here frees nothing because nothing was held.
//
// The returned guest keeps the room fresh in the background, so a screen
// showing the roster sees somebody else arrive without asking.
func MatchAt[W, A, D, S any](ctx context.Context, opts Options[W, A, D, S], endpoint string) (*Guest[W, A, D, S], error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	room, err := fetchRoom(ctx, endpoint, opts.Protocol)
	if err != nil {
		return nil, err
	}
	watching, stop := context.WithCancel(context.WithoutCancel(ctx))
	g := &Guest[W, A, D, S]{
		opts:     opts,
		endpoint: endpoint,
		room:     room,
		ready:    make(chan struct{}),
		stop:     stop,
	}
	go g.watch(watching)
	return g, nil
}

// watch re-reads the room while this instance is only looking. It stops
// at the first sit: from then on the roster arrives over the link.
func (g *Guest[W, A, D, S]) watch(ctx context.Context) {
	t := time.NewTicker(roomRefresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			room, err := fetchRoom(ctx, g.endpoint, g.opts.Protocol)
			if err != nil {
				continue // a blink on the segment is not a departure
			}
			g.mu.Lock()
			if g.seated {
				g.mu.Unlock()
				return
			}
			g.room = room
			g.mu.Unlock()
		}
	}
}

// Room reports what this instance last saw of the room.
func (g *Guest[W, A, D, S]) Room() run.Room {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.room
}

// Seated reports whether this instance holds a seat.
func (g *Guest[W, A, D, S]) Seated() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.seated
}

// Sit asks the room for a seat and opens the link once it has one. It
// returns before the host has started anything, because the host is still
// gathering and a guest with nothing on screen until then would look
// broken. The handshake happens in Play.
func (g *Guest[W, A, D, S]) Sit(ctx context.Context) error {
	g.mu.Lock()
	if g.seated {
		g.mu.Unlock()
		return errors.New("lan: this instance already holds a seat")
	}
	g.mu.Unlock()

	grant, err := requestSeat(ctx, g.endpoint, g.opts.Protocol)
	if err != nil {
		return err
	}
	conn, err := ws.Dial(ctx, "ws://"+g.endpoint+"/ws")
	if err != nil {
		return fmt.Errorf("lan: dialling %s: %w", g.endpoint, err)
	}
	g.mu.Lock()
	g.conn, g.ticket, g.seat, g.seated = conn, grant.Ticket, session.SlotID(grant.Seat), true
	g.mu.Unlock()
	g.stop()
	return nil
}

// fetchRoom reads the room without asking for anything.
func fetchRoom(ctx context.Context, endpoint, protocol string) (run.Room, error) {
	var room run.Room
	if err := getJSON(ctx, "http://"+endpoint+"/room?protocol="+protocol, &room); err != nil {
		return run.Room{}, fmt.Errorf("lan: reading the room at %s: %w", endpoint, err)
	}
	return room, nil
}

// requestSeat asks the host's own control plane for a seat.
func requestSeat(ctx context.Context, endpoint, protocol string) (grant, error) {
	var g grant
	if err := getJSON(ctx, "http://"+endpoint+"/sit?protocol="+protocol, &g); err != nil {
		return grant{}, fmt.Errorf("lan: asking %s for a seat: %w", endpoint, err)
	}
	return g, nil
}

// getJSON is the one shape both control-plane calls take.
func getJSON(ctx context.Context, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

// Slot is the seat the host granted. It is known before the handshake,
// which is what lets a guest draw its own side of the board while it is
// still waiting.
func (g *Guest[W, A, D, S]) Slot() session.SlotID { return g.seat }

// Ready closes once the handshake completes and state starts arriving.
func (g *Guest[W, A, D, S]) Ready() <-chan struct{} { return g.ready }

// Seed is the match's shared RNG seed, delivered by the handshake. It is
// meaningful only after Ready closes.
func (g *Guest[W, A, D, S]) Seed() uint64 {
	if c := g.live(); c != nil {
		return c.Seed
	}
	return 0
}

// State is the newest world this guest has reconstructed.
func (g *Guest[W, A, D, S]) State() (*W, session.Tick, bool) {
	c := g.live()
	if c == nil {
		var zero *W
		return zero, 0, false
	}
	return c.State()
}

// Run completes the handshake, then drives the agent until the link
// ends. The wait inside the handshake is the host's lobby: it admits
// when its match begins.
//
// The agent is an ordinary one: on this side of the link the person at
// the keyboard and a bot are the same kind of object, which is what
// makes replacing one with the other a seating decision rather than a
// rewrite.
func (g *Guest[W, A, D, S]) Run(ctx context.Context, agent session.Agent[S, A]) error {
	client, err := netplay.Connect(ctx, g.conn, g.ticket, netplay.ClientConfig[W, A, D, S]{
		Protocol:    g.opts.Protocol,
		Tuning:      g.opts.Tuning,
		Codec:       g.opts.Codec,
		EncodeInput: g.opts.EncodeInput,
		Project:     g.opts.Project,
	})
	if err != nil {
		return err
	}
	g.mu.Lock()
	g.client = client
	g.mu.Unlock()
	close(g.ready)
	return client.Run(ctx, agent)
}

func (g *Guest[W, A, D, S]) live() *netplay.Client[W, A, D, S] {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.client
}

// Close leaves, whether or not the handshake ever completed. From an
// instance that only matched there is no link and no seat to give back,
// so it stops watching the room and returns.
func (g *Guest[W, A, D, S]) Close() error {
	g.stop()
	if c := g.live(); c != nil {
		return c.Close()
	}
	g.mu.Lock()
	conn := g.conn
	g.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// advertisedHost picks the address a peer on the same segment can reach.
// It opens no connection: a udp socket only needs a route to name a
// local address.
func advertisedHost() (string, error) {
	c, err := net.Dial("udp4", "192.0.2.1:9")
	if err != nil {
		return "127.0.0.1", nil // no route off the box; loopback still plays
	}
	defer c.Close()
	host, _, err := net.SplitHostPort(c.LocalAddr().String())
	if err != nil {
		return "", err
	}
	return host, nil
}
