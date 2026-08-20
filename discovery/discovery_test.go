//go:build !js && !wasip1

package discovery

import (
	"context"
	"testing"
	"time"
)

// Announcer and listener meet over loopback: 127.0.0.1 only, never the
// real broadcast address, so CI machines (macOS firewall prompts!) stay
// quiet.
func TestDiscoveryOverLoopback(t *testing.T) {
	l, err := Listen("127.0.0.1:0", "proto-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	players := 1
	a := &Announcer{
		Addr:     l.Addr().String(),
		Interval: 50 * time.Millisecond,
		Beacon: func() Beacon {
			return Beacon{
				Session:         "friday-pong",
				Endpoint:        "192.168.1.10:4433",
				ProtocolVersion: "proto-v1",
				PlayerCount:     players,
				TicketRequired:  false,
			}
		},
	}
	go func() { _ = a.Run(ctx) }()

	// A mismatched build announces too; it must stay invisible
	// (rule:protocol-version-must-match).
	stale := &Announcer{
		Addr:     l.Addr().String(),
		Interval: 50 * time.Millisecond,
		Beacon: func() Beacon {
			return Beacon{
				Session:         "old-build",
				Endpoint:        "192.168.1.99:4433",
				ProtocolVersion: "proto-v0",
			}
		},
	}
	go func() { _ = stale.Run(ctx) }()

	deadline := time.After(time.Second)
	for {
		sessions := l.Sessions()
		if len(sessions) == 1 {
			b := sessions[0]
			if b.Session != "friday-pong" || b.Endpoint != "192.168.1.10:4433" || b.PlayerCount != 1 || b.TicketRequired {
				t.Fatalf("beacon = %+v", b)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("discovered %d sessions within a second, want exactly 1 (version filter)", len(sessions))
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Give the mismatched announcer time to have landed too; it must
	// still be hidden.
	time.Sleep(120 * time.Millisecond)
	for _, b := range l.Sessions() {
		if b.ProtocolVersion != "proto-v1" {
			t.Fatalf("version-mismatched beacon leaked: %+v", b)
		}
		if b.Session == "old-build" {
			t.Fatalf("old-build must be hidden")
		}
	}
}

// Quiet sessions age out of the listing.
func TestBeaconExpiry(t *testing.T) {
	l, err := Listen("127.0.0.1:0", "proto-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.TTL = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	a := &Announcer{
		Addr:     l.Addr().String(),
		Interval: 20 * time.Millisecond,
		Beacon: func() Beacon {
			return Beacon{Session: "s", Endpoint: "10.0.0.1:1", ProtocolVersion: "proto-v1"}
		},
	}
	go func() { _ = a.Run(ctx) }()

	deadline := time.After(time.Second)
	for len(l.Sessions()) == 0 {
		select {
		case <-deadline:
			t.Fatal("never discovered")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel() // announcer goes quiet
	time.Sleep(250 * time.Millisecond)
	if got := l.Sessions(); len(got) != 0 {
		t.Fatalf("stale sessions still listed: %+v", got)
	}
}
