package signaltoken

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// sampleSDP is shaped like a real pion data-channel offer.
const sampleSDP = "v=0\r\n" +
	"o=- 4611731400430051336 2 IN IP4 127.0.0.1\r\n" +
	"s=-\r\nt=0 0\r\n" +
	"a=group:BUNDLE 0\r\n" +
	"m=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\n" +
	"c=IN IP4 0.0.0.0\r\n" +
	"a=ice-ufrag:aBcDeFgH\r\n" +
	"a=ice-pwd:aBcDeFgHiJkLmNoPqRsTuVwX\r\n" +
	"a=fingerprint:sha-256 00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF\r\n" +
	"a=setup:actpass\r\n" +
	"a=mid:0\r\n" +
	"a=sctp-port:5000\r\n" +
	"a=max-message-size:65536\r\n" +
	"a=candidate:foundation 1 udp 2130706431 192.168.1.10 51234 typ host\r\n"

func encode(t *testing.T) (string, Payload) {
	t.Helper()
	p := Payload{SessionID: "pong-1", SDP: sampleSDP}
	tok, err := Encode(Invitation, p, time.Now().Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return tok, p
}

func TestRoundTrip(t *testing.T) {
	tok, p := encode(t)
	if idx := alphabetEnd(tok); idx != len(tok) {
		t.Fatalf("token is not text-safe at %d", idx)
	}
	got, err := Decode(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != Invitation || got.Payload != p {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	// The token must be pasteable: notably shorter than the raw SDP.
	if len(tok) >= len(sampleSDP) {
		t.Errorf("token (%d) not smaller than SDP (%d)", len(tok), len(sampleSDP))
	}

	ans, err := Encode(Answer, p, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Decode(ans); err != nil || got.Type != Answer {
		t.Fatalf("answer decode: %+v err=%v", got, err)
	}
}

func TestTrailingGarbageTolerated(t *testing.T) {
	tok, p := encode(t)
	// Junk outside the alphabet is cut; junk inside the alphabet decodes
	// into bytes beyond the declared length, which are not read.
	for _, junk := range []string{")", " was pasted into chat!", "...", "AAAAAAAA", "_-42"} {
		got, err := Decode(tok + junk)
		if err != nil {
			t.Fatalf("junk %q: %v", junk, err)
		}
		if got.Payload != p {
			t.Fatalf("junk %q corrupted payload", junk)
		}
	}
}

// tamper decodes the token binary, applies f, and re-encodes.
func tamper(t *testing.T, tok string, f func([]byte) []byte) string {
	t.Helper()
	bin, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(f(bin))
}

func TestValidationFailures(t *testing.T) {
	tok, _ := encode(t)

	t.Run("wrong version", func(t *testing.T) {
		bad := tamper(t, tok, func(b []byte) []byte { b[0] = 9; return b })
		if _, err := Decode(bad); !errors.Is(err, ErrVersion) {
			t.Fatalf("err = %v, want ErrVersion", err)
		}
	})
	t.Run("unknown type", func(t *testing.T) {
		bad := tamper(t, tok, func(b []byte) []byte { b[1] = 7; return b })
		if _, err := Decode(bad); !errors.Is(err, ErrType) {
			t.Fatalf("err = %v, want ErrType", err)
		}
	})
	t.Run("tampered length", func(t *testing.T) {
		// Declared length beyond the actual bytes: truncated payload.
		bad := tamper(t, tok, func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[10:14], uint32(len(b))) // > available
			return b
		})
		if _, err := Decode(bad); !errors.Is(err, ErrMalformed) {
			t.Fatalf("err = %v, want ErrMalformed", err)
		}
		// Declared length over the inflate bound.
		bad = tamper(t, tok, func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[10:14], maxInflated+1)
			return b
		})
		if _, err := Decode(bad); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err = %v, want ErrTooLarge", err)
		}
	})
	t.Run("expired", func(t *testing.T) {
		old, err := Encode(Invitation, Payload{SessionID: "pong-1", SDP: sampleSDP}, time.Now().Add(-time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(old); !errors.Is(err, ErrExpired) {
			t.Fatalf("err = %v, want ErrExpired", err)
		}
	})
	t.Run("truncated token", func(t *testing.T) {
		if _, err := Decode(tok[:10]); !errors.Is(err, ErrMalformed) {
			t.Fatalf("err = %v, want ErrMalformed", err)
		}
	})
	t.Run("corrupt payload", func(t *testing.T) {
		bad := tamper(t, tok, func(b []byte) []byte { b[headerLen+2] ^= 0xFF; return b })
		if _, err := Decode(bad); err == nil {
			t.Fatal("corrupted payload accepted")
		}
	})
	t.Run("bad sdp shape rejected at encode", func(t *testing.T) {
		if _, err := Encode(Invitation, Payload{SessionID: "s", SDP: "hello"}, time.Now().Add(time.Minute)); !errors.Is(err, ErrMalformed) {
			t.Fatalf("err = %v, want ErrMalformed", err)
		}
	})
}

func TestRedeemerSingleUse(t *testing.T) {
	tok, _ := encode(t)
	dec, err := Decode(tok)
	if err != nil {
		t.Fatal(err)
	}
	r := &Redeemer{}
	if err := r.Redeem(tok, dec.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	if err := r.Redeem(tok, dec.ExpiresAt); !errors.Is(err, ErrRedeemed) {
		t.Fatalf("second redeem: %v, want ErrRedeemed", err)
	}
	// A different token is unaffected.
	other, _ := Encode(Answer, Payload{SessionID: "pong-2", SDP: sampleSDP}, time.Now().Add(time.Minute))
	if err := r.Redeem(other, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	// Expired entries are pruned, so memory stays bounded.
	now := time.Now()
	r2 := &Redeemer{Now: func() time.Time { return now }}
	if err := r2.Redeem(tok, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	if len(r2.used) != 1 {
		t.Fatalf("used len = %d", len(r2.used))
	}
	if err := r2.Redeem(other, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, stale := r2.used[sha256.Sum256([]byte(tok))]; stale {
		t.Fatal("expired entry not pruned")
	}
}
