package pong_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/samples/pong/pong"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/wt"
)

// selfSigned issues a throwaway localhost certificate for the test
// server.
func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// The full stack over real WebTransport on localhost: QUIC datagrams for
// state and input, one reliable stream for handshake and resync — the
// datagram-capable primary transport
// (decision:webtransport-primary-for-wasm).
func TestPongOverWebTransport(t *testing.T) {
	cert := selfSigned(t)

	// Pick a free UDP port up front so the client knows the URL.
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	pc.Close()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	accepted := make(chan transport.Conn, 2)
	mux := http.NewServeMux()
	var server *wt.Server
	mux.HandleFunc("/pong", func(w http.ResponseWriter, r *http.Request) {
		conn, err := server.Upgrade(w, r)
		if err != nil {
			t.Logf("upgrade: %v", err)
			return
		}
		accepted <- conn
	})
	server = wt.NewServer(addr, &tls.Config{Certificates: []tls.Certificate{cert}}, mux)
	defer server.Close()
	go func() { _ = server.ListenAndServe() }()

	tuning := session.TuningProfile{
		TickRate: 120, SendRate: 60, HistoryDepth: 32, SnapshotEvery: 0,
		BaselineMode: session.BaselineBounded, SpeculationDepth: 8,
		AckMode: 2,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://%s/pong", addr)
	clientConns := map[session.SlotID]transport.Conn{}
	serverConns := map[session.SlotID]transport.Conn{}
	var mu sync.Mutex
	var dialWg sync.WaitGroup
	for _, slot := range pong.Slots() {
		dialWg.Add(1)
		go func(slot session.SlotID) {
			defer dialWg.Done()
			conn, err := wt.Dial(ctx, url, &tls.Config{InsecureSkipVerify: true})
			if err != nil {
				t.Errorf("dial slot %d: %v", slot, err)
				return
			}
			if !conn.Capability().UnreliableDatagram {
				t.Error("webtransport must offer real datagrams")
			}
			mu.Lock()
			clientConns[slot] = conn
			mu.Unlock()
		}(slot)
	}
	dialWg.Wait()
	if t.Failed() {
		return
	}
	// Pair accepted server conns with slots in dial order; which slot
	// got which connection does not matter — the ticket names the seat.
	for _, slot := range pong.Slots() {
		select {
		case conn := <-accepted:
			serverConns[slot] = conn
		case <-ctx.Done():
			t.Fatal("server never accepted both sessions")
		}
	}

	s, clients := runNetworkMatch(t, tuning, serverConns, clientConns, 1500*time.Millisecond)
	if s.State() != session.StateEnded {
		t.Fatalf("session state = %v, want ended", s.State())
	}
	for slot, nc := range clients {
		_, tick, ok := nc.State()
		if !ok || tick < s.Tick()/2 {
			t.Errorf("slot %d: reconstructed to tick %d of %d (ok=%v)", slot, tick, s.Tick(), ok)
		}
		if nc.Stats().RTT <= 0 {
			t.Errorf("slot %d: no RTT sample over webtransport", slot)
		}
	}
}
