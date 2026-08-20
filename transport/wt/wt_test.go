//go:build !js && !wasip1

package wt_test

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
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/wt"
)

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
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func TestEchoBothChannels(t *testing.T) {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	pc.Close()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	accepted := make(chan *wt.Conn, 1)
	mux := http.NewServeMux()
	var server *wt.Server
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		conn, err := server.Upgrade(w, r)
		if err != nil {
			t.Logf("upgrade: %v", err)
			return
		}
		accepted <- conn
	})
	server = wt.NewServer(addr, &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}}, mux)
	defer server.Close()
	go func() { _ = server.ListenAndServe() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := wt.Dial(ctx, "https://"+addr+"/echo", &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var serverConn transport.Conn
	select {
	case serverConn = <-accepted:
	case <-ctx.Done():
		t.Fatal("server never accepted")
	}
	defer serverConn.Close()

	if !client.Capability().UnreliableDatagram || !client.Capability().ReliableStream {
		t.Fatalf("capability = %+v", client.Capability())
	}

	// Reliable round trip preserves message boundaries.
	for i := range 3 {
		msg := []byte(fmt.Sprintf("hello-%d", i))
		if err := client.SendReliable(ctx, msg); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 3 {
		m, err := serverConn.Receive(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if m.Channel != transport.Reliable || string(m.Payload) != fmt.Sprintf("hello-%d", i) {
			t.Fatalf("got %v %q", m.Channel, m.Payload)
		}
	}

	// Datagram round trip, server → client.
	if err := serverConn.SendUnreliable(ctx, []byte("dgram")); err != nil {
		t.Fatal(err)
	}
	m, err := client.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m.Channel != transport.Unreliable || string(m.Payload) != "dgram" {
		t.Fatalf("got %v %q", m.Channel, m.Payload)
	}
}
