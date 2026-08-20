package framing_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/framing"
	"github.com/shibukawa/ebigentserver/transport/pipe"
)

func newPair(t *testing.T, limits framing.Limits) (*framing.Framer, *framing.Framer, transport.Conn, transport.Conn) {
	t.Helper()
	a, b := pipe.Pair(pipe.Faults{}, pipe.Faults{})
	t.Cleanup(func() { a.Close(); b.Close() })
	fa, err := framing.New(a, limits)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := framing.New(b, limits)
	if err != nil {
		t.Fatal(err)
	}
	return fa, fb, a, b
}

var limits = framing.Limits{ChunkSize: 64, MaxMessageSize: 1024, MaxPending: 4}

func recvOne(t *testing.T, fr *framing.Framer, conn transport.Conn) []byte {
	t.Helper()
	ctx := context.Background()
	for range 100 {
		m, err := conn.Receive(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out, err := fr.Absorb(m.Payload)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			return out
		}
	}
	t.Fatal("message never completed")
	return nil
}

func TestChunkAndReassemble(t *testing.T) {
	fa, fb, _, connB := newPair(t, limits)
	// 300 bytes over 64-byte chunks = 5 frames.
	msg := bytes.Repeat([]byte("abcdefghij"), 30)
	if err := fa.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	got := recvOne(t, fb, connB)
	if !bytes.Equal(got, msg) {
		t.Fatalf("reassembled %d bytes, want %d", len(got), len(msg))
	}
	// A second message still works (ids advance).
	if err := fa.Send(context.Background(), []byte("small")); err != nil {
		t.Fatal(err)
	}
	if got := recvOne(t, fb, connB); string(got) != "small" {
		t.Fatalf("second message = %q", got)
	}
}

func TestOversizedSendRefused(t *testing.T) {
	fa, _, _, _ := newPair(t, limits)
	err := fa.Send(context.Background(), make([]byte, 2048))
	if err == nil {
		t.Fatal("oversized send accepted")
	}
}

func TestMalformedFramesAreDropped(t *testing.T) {
	fr, err := framing.New(nil, limits)
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range [][]byte{
		nil,
		{0x00},
		[]byte("definitely not a frame"),
		{0xEB, 1, 0, 0, 0, 1, 0, 9, 0, 2}, // index 9 of count 2: out of range
	} {
		if out, err := fr.Absorb(frame); err != nil || out != nil {
			t.Errorf("frame %v: out=%v err=%v, want dropped", frame, out, err)
		}
	}
}

func TestPartialFloodIsBounded(t *testing.T) {
	fr, err := framing.New(nil, limits)
	if err != nil {
		t.Fatal(err)
	}
	// Open MaxPending+2 partial reassemblies; the extras are dropped and
	// completing an accepted one still works.
	frame := func(id uint32, index, count uint16, payload string) []byte {
		f := []byte{0xEB, 1, byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id),
			byte(index >> 8), byte(index), byte(count >> 8), byte(count)}
		return append(f, payload...)
	}
	for id := uint32(1); id <= 6; id++ {
		if out, _ := fr.Absorb(frame(id, 0, 2, "x")); out != nil {
			t.Fatal("half message completed")
		}
	}
	// ids 5 and 6 were over the pending bound: their second halves lead
	// nowhere.
	if out, _ := fr.Absorb(frame(6, 1, 2, "y")); out != nil {
		t.Error("over-bound reassembly completed")
	}
	// id 1 was accepted and completes.
	out, _ := fr.Absorb(frame(1, 1, 2, "y"))
	if string(out) != "xy" {
		t.Errorf("accepted reassembly = %q, want xy", out)
	}
}
