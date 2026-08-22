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

// Options declares what a game puts on the wire. Every field but the
// test overrides is required: this package encodes nothing on its own,
// it only carries what the game's generated codec produces.
type Options[S, A, D, O any] struct {
	// Name is what the browse list shows.
	Name string
	// Protocol is data:protocol-version, compared before anything else
	// (rule:protocol-version-must-match).
	Protocol string
	// Codec is the game's generated snapshot and delta encoding.
	Codec statesync.Codec[S, D]
	// Tuning is the declared data:session-tuning-profile.
	Tuning session.TuningProfile
	// Budget bounds connections and input rate. The zero value takes
	// the package default, which suits a handful of players.
	Budget budget.Budget
	// EncodeInput and DecodeInput carry data:player-input.
	EncodeInput func(dst []byte, a A) []byte
	DecodeInput func(b []byte) (A, error)
	// Project builds a slot's observation on the guest side, from the
	// world the guest reconstructed.
	Project func(world *S, slot session.SlotID) O
	// Port is the host's listening port; 0 picks a free one.
	Port int

	// BeaconAddr overrides the broadcast destination and DiscoveryAddr
	// the listen address. Both are for tests, which cannot broadcast.
	BeaconAddr    string
	DiscoveryAddr string
}

func (o Options[S, A, D, O]) validate() error {
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

func (o Options[S, A, D, O]) budgetOrDefault() budget.Budget {
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
type Host[S, A, D, O any] struct {
	opts   Options[S, A, D, O]
	roster *run.Roster[S, A, O]

	endpoint string
	priv     ed25519.PrivateKey
	verifier *admission.Verifier
	seed     uint64

	listener net.Listener
	http     *http.Server
	upgrader *ws.Upgrader

	mu        sync.Mutex
	parked    []transport.Conn
	closed    bool
	reportErr func(error)

	server atomic.Pointer[netplay.Server[S, A]]

	stop  context.CancelFunc
	wg    sync.WaitGroup
	peers sync.WaitGroup
}

// Open starts listening and announcing. The roster is the one the lobby
// is filling: a guest reaching /join takes a seat in it, so the lobby
// shows the arrival without knowing a network exists.
func Open[S, A, D, O any](ctx context.Context, opts Options[S, A, D, O], roster *run.Roster[S, A, O], seed uint64) (*Host[S, A, D, O], error) {
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
	h := &Host[S, A, D, O]{
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
	mux.HandleFunc("/join", h.handleJoin)
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
func (h *Host[S, A, D, O]) Endpoint() string { return h.endpoint }

// announce repeats the beacon until the context ends.
func (h *Host[S, A, D, O]) announce(ctx context.Context) {
	a := &discovery.Announcer{
		Addr: h.opts.BeaconAddr,
		Beacon: func() discovery.Beacon {
			seated := 0
			for _, s := range h.roster.Seats() {
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

// handleJoin seats the caller in the roster and mints the ticket naming
// that seat. This is the whole of the control plane, and it lives inside
// the host process because on a segment there is nothing else to ask.
func (h *Host[S, A, D, O]) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("protocol") != h.opts.Protocol {
		http.Error(w, "protocol mismatch: host speaks "+h.opts.Protocol, http.StatusConflict)
		return
	}
	slot, err := h.claimFreeSeat(r.RemoteAddr)
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
		h.roster.Leave(slot)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(grant{Ticket: ticket, Seat: uint16(slot)})
}

// claimFreeSeat takes the lowest seat nobody holds. Two guests arriving
// together race here, and the roster's own lock decides.
func (h *Host[S, A, D, O]) claimFreeSeat(id string) (session.SlotID, error) {
	for _, seat := range h.roster.Seats() {
		if seat.Filled() {
			continue
		}
		if err := h.roster.JoinRemote(seat.Slot, id); err == nil {
			return seat.Slot, nil
		}
	}
	return 0, errors.New("lan: every seat is taken")
}

// handleWS parks an upgraded connection. It is not admitted yet: the
// session it would be admitted into does not exist until the lobby
// starts the match.
func (h *Host[S, A, D, O]) handleWS(w http.ResponseWriter, r *http.Request) {
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
func (h *Host[S, A, D, O]) Attach(cfg *session.Config[S, A, O]) {
	prev := cfg.Broadcast
	cfg.Broadcast = func(tick session.Tick, world *S) {
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
func (h *Host[S, A, D, O]) Serve(ctx context.Context, m *run.Match[S, A, O]) error {
	if m == nil {
		return errors.New("lan: Serve needs the finalized match")
	}
	b := h.opts.budgetOrDefault()
	sv, err := netplay.NewServer(ctx, netplay.ServerConfig[S, A]{
		SessionID: h.opts.Name,
		Protocol:  h.opts.Protocol,
		Verifier:  h.verifier,
		Seed:      h.seed,
		Tuning:    h.opts.Tuning,
		Budget:    b,
		MakeSender: func(session.SlotID, string) (statesync.ViewSender[S], error) {
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
	return nil
}

// admit completes one handshake and starts the peer's own loop, which is
// what carries data:player-input the other way. Without it a guest is
// seen and sent to, and never heard.
//
// It runs on its own goroutine because the handshake waits on the other
// end, and the match must not: a guest that never says hello is a seat
// that stays silent, not a match that never starts.
func (h *Host[S, A, D, O]) admit(ctx context.Context, sv *netplay.Server[S, A], conn transport.Conn) {
	peer, err := sv.Admit(ctx, conn)
	if err != nil {
		_ = conn.Close()
		if f := h.onError(); f != nil {
			f(err)
		}
		return
	}
	h.peers.Add(1)
	go peer.Run(ctx, &h.peers)
}

// onError reports the sink for admission failures, if the game set one.
func (h *Host[S, A, D, O]) onError() func(error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.reportErr
}

// OnError registers a sink for admission failures — a guest whose
// protocol drifted, or one that went away between taking a seat and
// saying hello. Without it they are silent, which is the right default
// for a game: one guest failing is not the match failing.
func (h *Host[S, A, D, O]) OnError(f func(error)) {
	h.mu.Lock()
	h.reportErr = f
	h.mu.Unlock()
}

// Close stops announcing and listening. Connections already admitted
// belong to the match and end with it.
func (h *Host[S, A, D, O]) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	waiting := h.parked
	h.parked = nil
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
func Browse[S, A, D, O any](ctx context.Context, opts Options[S, A, D, O], window time.Duration) ([]discovery.Beacon, error) {
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
type Guest[S, A, D, O any] struct {
	opts   Options[S, A, D, O]
	conn   transport.Conn
	ticket string
	seat   session.SlotID
	box    mailbox[A]

	ready chan struct{}

	mu      sync.Mutex
	client  *netplay.Client[S, A, D, O]
	onWorld func(session.Tick, *S)
	over    bool
}

// Join takes a seat on a host. It returns as soon as the seat is
// granted and the link is open — before the host has started anything,
// because the host is still gathering and a guest with nothing on screen
// until then would look broken. The handshake happens in Play.
func Join[S, A, D, O any](ctx context.Context, opts Options[S, A, D, O], endpoint string) (*Guest[S, A, D, O], error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	g, err := requestSeat(ctx, endpoint, opts.Protocol)
	if err != nil {
		return nil, err
	}
	conn, err := ws.Dial(ctx, "ws://"+endpoint+"/ws")
	if err != nil {
		return nil, fmt.Errorf("lan: dialling %s: %w", endpoint, err)
	}
	return &Guest[S, A, D, O]{
		opts:   opts,
		conn:   conn,
		ticket: g.Ticket,
		seat:   session.SlotID(g.Seat),
		ready:  make(chan struct{}),
	}, nil
}

// requestSeat asks the host's own control plane for a seat.
func requestSeat(ctx context.Context, endpoint, protocol string) (grant, error) {
	url := "http://" + endpoint + "/join?protocol=" + protocol
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return grant{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return grant{}, fmt.Errorf("lan: asking %s for a seat: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return grant{}, fmt.Errorf("lan: %s refused a seat: %s", endpoint, resp.Status)
	}
	var g grant
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return grant{}, err
	}
	return g, nil
}

// Slot is the seat the host granted. It is known before the handshake,
// which is what lets a guest draw its own side of the board while it is
// still waiting.
func (g *Guest[S, A, D, O]) Slot() session.SlotID { return g.seat }

// Ready closes once the handshake completes and state starts arriving.
func (g *Guest[S, A, D, O]) Ready() <-chan struct{} { return g.ready }

// Seed is the match's shared RNG seed, delivered by the handshake. It is
// meaningful only after Ready closes.
func (g *Guest[S, A, D, O]) Seed() uint64 {
	if c := g.live(); c != nil {
		return c.Seed
	}
	return 0
}

// State is the newest world this guest has reconstructed.
func (g *Guest[S, A, D, O]) State() (*S, session.Tick, bool) {
	c := g.live()
	if c == nil {
		var zero *S
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
func (g *Guest[S, A, D, O]) Run(ctx context.Context, agent session.Agent[O, A]) error {
	client, err := netplay.Connect(ctx, g.conn, g.ticket, netplay.ClientConfig[S, A, D, O]{
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

func (g *Guest[S, A, D, O]) live() *netplay.Client[S, A, D, O] {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.client
}

// Close ends the link, whether or not the handshake ever completed.
func (g *Guest[S, A, D, O]) Close() error {
	if c := g.live(); c != nil {
		return c.Close()
	}
	return g.conn.Close()
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
