package seqack_test

import (
	"context"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/pipe"
	"github.com/shibukawa/ebigentserver/transport/seqack"
)

func pair(t *testing.T, opts seqack.Options) (la, lb *seqack.Layer, ca, cb transport.Conn) {
	t.Helper()
	ca, cb = pipe.Pair(pipe.Faults{}, pipe.Faults{})
	t.Cleanup(func() { ca.Close(); cb.Close() })
	return seqack.New(ca, opts), seqack.New(cb, opts), ca, cb
}

func pump(t *testing.T, l *seqack.Layer, conn transport.Conn) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		m, err := conn.Receive(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if payload := l.Absorb(m.Payload); payload != nil {
			return payload
		}
	}
}

// A piggybacked round trip confirms the tag: a→b payload, b→a payload,
// whose ack record tells a that b holds tag 100.
func TestPiggybackConfirmsTag(t *testing.T) {
	la, lb, ca, cb := pair(t, seqack.Options{Policy: seqack.PiggybackOnly})
	ctx := context.Background()

	if err := la.SendDatagram(ctx, []byte("state-100"), 100); err != nil {
		t.Fatal(err)
	}
	if got := pump(t, lb, cb); string(got) != "state-100" {
		t.Fatalf("b received %q", got)
	}
	if _, ok := la.Confirmed(); ok {
		t.Fatal("a confirmed before any return traffic")
	}
	if err := lb.SendDatagram(ctx, []byte("input-1"), 0); err != nil {
		t.Fatal(err)
	}
	if got := pump(t, la, ca); string(got) != "input-1" {
		t.Fatalf("a received %q", got)
	}
	tag, ok := la.Confirmed()
	if !ok || tag != 100 {
		t.Fatalf("confirmed = %d, %v; want 100, true", tag, ok)
	}
	if la.Stats().RTT <= 0 {
		t.Error("no RTT sample after a confirmed round trip")
	}
}

// Dedicated acks confirm without any return payload — the spectator case.
func TestDedicatedAckConfirms(t *testing.T) {
	la, lb, ca, cb := pair(t, seqack.Options{Policy: seqack.Dedicated})
	ctx := context.Background()

	if err := la.SendDatagram(ctx, []byte("state-7"), 7); err != nil {
		t.Fatal(err)
	}
	if got := pump(t, lb, cb); string(got) != "state-7" {
		t.Fatalf("b received %q", got)
	}
	if err := lb.MaybeFlushAck(ctx); err != nil {
		t.Fatal(err)
	}
	// The dedicated ack carries no payload; Absorb returns nil but the
	// confirmation lands.
	ctx2, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	m, err := ca.Receive(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	if payload := la.Absorb(m.Payload); payload != nil {
		t.Fatalf("dedicated ack delivered a payload: %q", payload)
	}
	if tag, ok := la.Confirmed(); !ok || tag != 7 {
		t.Fatalf("confirmed = %d, %v; want 7, true", tag, ok)
	}
}

// Stale datagrams are acked but not delivered: the state stream is
// last-writer-wins.
func TestStaleDatagramNotDelivered(t *testing.T) {
	la, lb, _, cb := pair(t, seqack.Options{Policy: seqack.PiggybackOnly})
	ctx := context.Background()

	// Send seq 0 and seq 1; deliver seq 1 first by absorbing out of
	// order (grab both raw frames first).
	if err := la.SendDatagram(ctx, []byte("old"), 1); err != nil {
		t.Fatal(err)
	}
	if err := la.SendDatagram(ctx, []byte("new"), 2); err != nil {
		t.Fatal(err)
	}
	rctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	m0, err := cb.Receive(rctx)
	if err != nil {
		t.Fatal(err)
	}
	m1, err := cb.Receive(rctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := lb.Absorb(m1.Payload); string(got) != "new" {
		t.Fatalf("newest first = %q, want new", got)
	}
	if got := lb.Absorb(m0.Payload); got != nil {
		t.Fatalf("stale datagram delivered: %q", got)
	}
}
