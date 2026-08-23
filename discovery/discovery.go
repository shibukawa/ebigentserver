//go:build !js && !wasip1

// Package discovery implements api:lan-discovery: a periodic UDP
// broadcast beacon that lets native clients find sessions on the same
// network without any server. The host announces; clients listen
// passively and list what responds, then connect through
// api:transport-interface.
//
// The scope is link-local by construction, which is the network_scope
// arm of rule:unauthenticated-admission-requires-scope-or-capability
// (pairs with decision:no-auth-on-lan). Beacons whose protocol version
// differs from the listener's are hidden outright
// (rule:protocol-version-must-match) — a stale build should see an
// empty lobby, not a join that fails late.
//
// Native builds only (rule:build-tag-only-for-linkage): browsers cannot
// send UDP broadcast, so wasm uses concept:static-host-mode instead.
package discovery

import (
	"context"
	"encoding/json"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// DefaultPort is the beacon port when none is configured.
const DefaultPort = 47777

// maxBeacon bounds one beacon datagram.
const maxBeacon = 1400

// Beacon is one announcement payload (api:lan-discovery
// beacon_payload).
type Beacon struct {
	// Session is the human-readable session name.
	Session string `json:"session"`
	// Endpoint is where to connect (host:port for the transport).
	Endpoint string `json:"endpoint"`
	// ProtocolVersion gates joinability
	// (rule:protocol-version-must-match).
	ProtocolVersion string `json:"protocol_version"`
	// PlayerCount is the current occupancy.
	PlayerCount int `json:"player_count"`
	// Terms are what this room stated when it opened
	// (requirement:conditional-matchmaking), so a browser can leave out
	// a room it cannot sit in rather than showing a row that refuses.
	Terms json.RawMessage `json:"terms,omitempty"`
	// TicketRequired says whether admission needs a
	// data:session-ticket; false is only legitimate under this
	// package's link-local scope.
	TicketRequired bool `json:"ticket_required"`
}

// Announcer broadcasts one host's beacon periodically.
type Announcer struct {
	// Addr is the destination; empty means limited broadcast on
	// DefaultPort ("255.255.255.255:47777"). Tests point it at a
	// loopback listener instead.
	Addr string
	// Interval between beacons; zero means 1s.
	Interval time.Duration
	// Beacon produces the current payload each tick, so player count
	// stays live.
	Beacon func() Beacon
}

// Run broadcasts until ctx is done.
func (a *Announcer) Run(ctx context.Context) error {
	addr := a.Addr
	if addr == "" {
		addr = net.JoinHostPort("255.255.255.255", strconv.Itoa(DefaultPort))
	}
	dest, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}
	lc := net.ListenConfig{Control: broadcastControl}
	pc, err := lc.ListenPacket(ctx, "udp4", ":0")
	if err != nil {
		return err
	}
	defer pc.Close()

	interval := a.Interval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		payload, err := json.Marshal(a.Beacon())
		if err != nil {
			return err
		}
		if _, err := pc.WriteTo(payload, dest); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Listener passively collects beacons.
type Listener struct {
	// TTL drops sessions whose beacon has gone quiet; the zero value
	// set by Listen is 5s.
	TTL time.Duration

	pc      net.PacketConn
	version string

	mu     sync.Mutex
	seen   map[string]entry // by endpoint
	closed sync.Once
}

type entry struct {
	b  Beacon
	at time.Time
}

// Listen starts collecting on addr (empty means ":47777");
// protocolVersion is this build's version — mismatched beacons are
// hidden, not listed as unjoinable (rule:protocol-version-must-match).
func Listen(addr, protocolVersion string) (*Listener, error) {
	if addr == "" {
		addr = ":" + strconv.Itoa(DefaultPort)
	}
	pc, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return nil, err
	}
	l := &Listener{
		TTL:     5 * time.Second,
		pc:      pc,
		version: protocolVersion,
		seen:    map[string]entry{},
	}
	go l.read()
	return l, nil
}

// Addr is the bound listen address (tests bind port 0).
func (l *Listener) Addr() net.Addr { return l.pc.LocalAddr() }

func (l *Listener) read() {
	buf := make([]byte, maxBeacon)
	for {
		n, _, err := l.pc.ReadFrom(buf)
		if err != nil {
			return // closed
		}
		var b Beacon
		if json.Unmarshal(buf[:n], &b) != nil {
			continue // not a beacon; ignore strangers on the port
		}
		if b.ProtocolVersion != l.version || b.Endpoint == "" {
			continue // version_filter: hidden, not shown-but-broken
		}
		l.mu.Lock()
		l.seen[b.Endpoint] = entry{b: b, at: time.Now()}
		l.mu.Unlock()
	}
}

// Sessions lists the currently-live sessions, freshest data per
// endpoint, stably ordered.
func (l *Listener) Sessions() []Beacon {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.TTL)
	out := make([]Beacon, 0, len(l.seen))
	for ep, e := range l.seen {
		if e.at.Before(cutoff) {
			delete(l.seen, ep)
			continue
		}
		out = append(out, e.b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Endpoint < out[j].Endpoint })
	return out
}

// Close stops listening; idempotent.
func (l *Listener) Close() error {
	l.closed.Do(func() { _ = l.pc.Close() })
	return nil
}
