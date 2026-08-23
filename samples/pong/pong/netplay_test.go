package pong_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/samples/pong/pong"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/pipe"
	"github.com/shibukawa/ebigentserver/transport/ws"
)

const testAudience = "sessions.test/pong-1"

func issuerKeys(t *testing.T) (ed25519.PrivateKey, *admission.Verifier) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return priv, &admission.Verifier{
		Keys:     map[string]ed25519.PublicKey{"k1": pub},
		Audience: testAudience,
		Leeway:   5 * time.Second,
	}
}

func ticket(t *testing.T, priv ed25519.PrivateKey, jti string, seat session.SlotID) string {
	t.Helper()
	tok, err := admission.Sign(priv, "k1", admission.Claims{
		Subject:   "player-" + jti,
		Audience:  testAudience,
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		ID:        jti,
		SessionID: "pong-1",
		Seat:      uint16(seat),
		Role:      "player",
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// Phase 3 completion criterion: a version-mismatched connection is
// rejected explicitly at the handshake (rule:protocol-version-must-match).
func TestVersionMismatchIsRejectedExplicitly(t *testing.T) {
	priv, verifier := issuerKeys(t)
	server, client := pipe.Pair(pipe.Faults{}, pipe.Faults{})
	defer server.Close()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		_, _ = admission.Accept(ctx, server, "server-proto-v2", verifier, 1)
	}()
	w, err := admission.Join(ctx, client, "client-proto-v1", ticket(t, priv, "j-ver", pong.SlotLeft))
	if !errors.Is(err, admission.ErrRejected) {
		t.Fatalf("Join error = %v, want ErrRejected", err)
	}
	if !strings.Contains(w.Reason, "protocol version mismatch") ||
		!strings.Contains(w.Reason, "server-proto-v2") || !strings.Contains(w.Reason, "client-proto-v1") {
		t.Fatalf("reason %q must name both versions", w.Reason)
	}
}

func TestTicketVerification(t *testing.T) {
	priv, verifier := issuerKeys(t)

	if _, err := verifier.Verify(ticket(t, priv, "ok-1", pong.SlotLeft)); err != nil {
		t.Fatalf("valid ticket rejected: %v", err)
	}
	// Replay: the same jti fails the second time
	// (rule:ticket-bound-to-connection).
	tok := ticket(t, priv, "replay-1", pong.SlotLeft)
	if _, err := verifier.Verify(tok); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(tok); !errors.Is(err, admission.ErrReplayed) {
		t.Fatalf("replayed jti: %v, want ErrReplayed", err)
	}
	// Expired.
	exp, err := admission.Sign(priv, "k1", admission.Claims{
		Audience: testAudience, ExpiresAt: time.Now().Add(-time.Hour).Unix(), ID: "old-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(exp); !errors.Is(err, admission.ErrExpired) {
		t.Fatalf("expired ticket: %v, want ErrExpired", err)
	}
	// Wrong audience.
	aud, err := admission.Sign(priv, "k1", admission.Claims{
		Audience: "sessions.test/other", ExpiresAt: time.Now().Add(time.Minute).Unix(), ID: "aud-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(aud); !errors.Is(err, admission.ErrAudience) {
		t.Fatalf("wrong audience: %v, want ErrAudience", err)
	}
	// Unknown kid.
	_, otherPriv, _ := ed25519.GenerateKey(nil)
	unk, err := admission.Sign(otherPriv, "k9", admission.Claims{
		Audience: testAudience, ExpiresAt: time.Now().Add(time.Minute).Unix(), ID: "kid-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(unk); !errors.Is(err, admission.ErrUnknownKey) {
		t.Fatalf("unknown kid: %v, want ErrUnknownKey", err)
	}
	// Tampered signature.
	if _, err := verifier.Verify(ticket(t, priv, "sig-1", pong.SlotLeft) + "x"); err == nil {
		t.Fatal("tampered ticket accepted")
	}
}

// runNetworkMatch drives a full match over the given per-player
// connections: admission, peers, session, bot clients.
func runNetworkMatch(t *testing.T, tuning session.TuningProfile, serverConns, clientConns map[session.SlotID]transport.Conn, d time.Duration) (*session.Session[pong.State, pong.Input, pong.Sight], map[session.SlotID]*pong.NetClient) {
	t.Helper()
	priv, verifier := issuerKeys(t)

	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	peers := pong.NewPeerSet(ctx)
	s := newSession(t, func(c *session.Config[pong.State, pong.Input, pong.Sight]) {
		c.Tuning = &tuning
		c.Seed = 11
		c.Broadcast = peers.Broadcast
	})

	var wg sync.WaitGroup
	clients := map[session.SlotID]*pong.NetClient{}
	var clientsMu sync.Mutex
	for _, slot := range pong.Slots() {
		// Server side: admit and run the peer.
		wg.Add(1)
		go func(slot session.SlotID) {
			defer wg.Done()
			peer, err := pong.AdmitRemote(ctx, serverConns[slot], s, verifier, 11, tuning)
			if err != nil {
				t.Errorf("admit slot %d: %v", slot, err)
				return
			}
			peers.Add(peer)
			wg.Add(1)
			go peer.Run(ctx, &wg)
		}(slot)
		// Client side: join and play.
		wg.Add(1)
		go func(slot session.SlotID) {
			defer wg.Done()
			nc, err := pong.Connect(ctx, clientConns[slot], ticket(t, priv, "net-"+t.Name()+string(rune('0'+slot)), slot), tuning)
			if err != nil {
				t.Errorf("connect slot %d: %v", slot, err)
				return
			}
			clientsMu.Lock()
			clients[slot] = nc
			clientsMu.Unlock()
			_ = nc.Run(ctx, &pong.Bot{})
		}(slot)
	}

	// Give both handshakes a moment before ticking starts.
	time.Sleep(150 * time.Millisecond)
	if err := s.RunRealtime(ctx, session.Paced); err != nil {
		t.Fatal(err)
	}
	cancel()
	for _, c := range serverConns {
		c.Close()
	}
	wg.Wait()
	return s, clients
}

// Phase 3 completion criterion: pong does not fall apart with loss and
// delay injected. 20% datagram loss, 25ms latency, 10ms jitter, and 5%
// reordering in both directions; the bounded-speculation baseline plus
// resync keeps every client reconstructing.
func TestPongSurvivesLossAndLatency(t *testing.T) {
	tuning := session.TuningProfile{
		TickRate: 120, SendRate: 60, HistoryDepth: 32, SnapshotEvery: 0,
		BaselineMode: session.BaselineBounded, SpeculationDepth: 8,
		AckMode: 2, // delayed piggyback
	}
	faults := pipe.Faults{LossPercent: 20, Latency: 25 * time.Millisecond, Jitter: 10 * time.Millisecond, ReorderPercent: 5, Seed: 42}
	serverConns := map[session.SlotID]transport.Conn{}
	clientConns := map[session.SlotID]transport.Conn{}
	for _, slot := range pong.Slots() {
		f := faults
		f.Seed += uint64(slot)
		sc, cc := pipe.Pair(f, f)
		serverConns[slot], clientConns[slot] = sc, cc
	}

	s, clients := runNetworkMatch(t, tuning, serverConns, clientConns, 3*time.Second)

	if s.State() != session.StateEnded {
		t.Fatalf("session state = %v, want ended", s.State())
	}
	total := s.Tick()
	if total < 200 {
		t.Fatalf("only %d ticks committed", total)
	}
	for slot, nc := range clients {
		_, tick, ok := nc.State()
		if !ok {
			t.Fatalf("slot %d never reconstructed a state", slot)
		}
		// The stream kept flowing under loss: the client's newest state
		// is close to the end of the match.
		if tick < total*7/10 {
			t.Errorf("slot %d fell behind: reached tick %d of %d", slot, tick, total)
		}
		stats := nc.Stats()
		if stats.RTT <= 0 {
			t.Errorf("slot %d: no RTT sample — acks never flowed", slot)
		}
		if stats.LossPercent == 0 {
			t.Errorf("slot %d: loss estimate is zero under 20%% injected loss", slot)
		}
	}
}

// The same stack over real WebSocket on localhost: the reliable-only
// fallback transport (rule:transport-selected-by-capability).
func TestPongOverWebSocket(t *testing.T) {
	tuning := session.TuningProfile{
		TickRate: 120, SendRate: 60, HistoryDepth: 16, SnapshotEvery: 60, AckMode: 2,
	}
	upgrader := ws.NewUpgrader()
	serverConns := map[session.SlotID]transport.Conn{}
	var mu sync.Mutex
	accepted := make(chan session.SlotID, 2)
	next := []session.SlotID{pong.SlotLeft, pong.SlotRight}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r)
		if err != nil {
			return
		}
		mu.Lock()
		slot := next[0]
		next = next[1:]
		serverConns[slot] = conn
		mu.Unlock()
		accepted <- slot
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	clientConns := map[session.SlotID]transport.Conn{}
	for range 2 {
		conn, err := ws.Dial(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		slot := <-accepted
		clientConns[slot] = conn
	}

	s, clients := runNetworkMatch(t, tuning, serverConns, clientConns, 1500*time.Millisecond)
	if s.State() != session.StateEnded {
		t.Fatalf("session state = %v, want ended", s.State())
	}
	for slot, nc := range clients {
		if _, tick, ok := nc.State(); !ok || tick < s.Tick()/2 {
			t.Errorf("slot %d: reconstructed to tick %d of %d (ok=%v)", slot, tick, s.Tick(), ok)
		}
	}
}
