//go:build !js && !wasip1

package rtc

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/transport"
)

// pair establishes a loopback WebRTC connection: offer → answer →
// complete, then waits for both Conns.
func pair(t *testing.T) (offerer, answerer transport.Conn, po, pa *Peer) {
	t.Helper()
	cfg := Config{IncludeLoopback: true, GatherTimeout: 5 * time.Second}
	po, offerSDP, err := NewOffer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pa, answerSDP, err := Accept(cfg, offerSDP)
	if err != nil {
		t.Fatal(err)
	}
	if err := po.Complete(answerSDP); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offerer, err = po.Conn(ctx)
	if err != nil {
		t.Fatalf("offerer conn: %v", err)
	}
	answerer, err = pa.Conn(ctx)
	if err != nil {
		t.Fatalf("answerer conn: %v", err)
	}
	return offerer, answerer, po, pa
}

func TestOfferAnswerRoundTrip(t *testing.T) {
	a, b, po, pa := pair(t)
	defer a.Close()
	defer b.Close()

	cap := a.Capability()
	if !cap.ReliableStream || !cap.UnreliableDatagram || !cap.PeerToPeer || !cap.Browser {
		t.Fatalf("capability = %+v, want full peer profile", cap)
	}
	if po.Fingerprint() == "" || pa.Fingerprint() == "" {
		t.Fatal("both sides must see a remote DTLS fingerprint")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Reliable both ways.
	if err := a.SendReliable(ctx, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	m, err := b.Receive(ctx)
	if err != nil || m.Channel != transport.Reliable || !bytes.Equal(m.Payload, []byte("ping")) {
		t.Fatalf("got %v %q err=%v", m.Channel, m.Payload, err)
	}
	if err := b.SendReliable(ctx, []byte("pong")); err != nil {
		t.Fatal(err)
	}
	if m, err = a.Receive(ctx); err != nil || !bytes.Equal(m.Payload, []byte("pong")) {
		t.Fatalf("got %q err=%v", m.Payload, err)
	}

	// Unreliable: retry until one datagram lands (delivery not
	// guaranteed per message, but loopback loses nothing in practice).
	got := make(chan transport.Message, 1)
	go func() {
		for {
			m, err := b.Receive(ctx)
			if err != nil {
				return
			}
			if m.Channel == transport.Unreliable {
				got <- m
				return
			}
		}
	}()
	deadline := time.After(4 * time.Second)
	for {
		if err := a.SendUnreliable(ctx, []byte("dgram")); err != nil {
			t.Fatal(err)
		}
		select {
		case m := <-got:
			if !bytes.Equal(m.Payload, []byte("dgram")) {
				t.Fatalf("datagram payload %q", m.Payload)
			}
			return
		case <-deadline:
			t.Fatal("no datagram delivered on loopback")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestErrorsAndClose(t *testing.T) {
	a, b, _, _ := pair(t)
	defer b.Close()

	ctx := context.Background()
	if err := a.SendReliable(ctx, make([]byte, maxMessage+1)); !errors.Is(err, transport.ErrTooLarge) {
		t.Fatalf("oversized send: %v, want ErrTooLarge", err)
	}

	// Close is idempotent and flips sends/receives to ErrClosed.
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.SendReliable(ctx, []byte("x")); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("send after close: %v, want ErrClosed", err)
	}
	if err := a.SendUnreliable(ctx, []byte("x")); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("send after close: %v, want ErrClosed", err)
	}
	if _, err := a.Receive(ctx); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("receive after close: %v, want ErrClosed", err)
	}
}
