//go:build !js && !wasip1

package pong_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/samples/pong/pong"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/signaltoken"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/rtc"
)

// rtcPair establishes one host↔joiner WebRTC connection with the whole
// serverless signaling path: the host's offer travels as an
// api:manual-signaling-token, the joiner redeems it (single use), and
// the answer comes back the same way — simulating the out-of-band
// chat/wiki exchange in-process.
func rtcPair(t *testing.T, redeemer *signaltoken.Redeemer) (host, joiner transport.Conn) {
	t.Helper()
	cfg := rtc.Config{IncludeLoopback: true, GatherTimeout: 5 * time.Second}

	// Host side: offer → invitation token.
	hp, offerSDP, err := rtc.NewOffer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	invitation, err := signaltoken.Encode(signaltoken.Invitation,
		signaltoken.Payload{SessionID: "pong-1", SDP: offerSDP},
		time.Now().Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	// Joiner side: decode (with chat-appended junk), redeem once,
	// answer back as a token.
	inv, err := signaltoken.Decode(invitation + "\n-- pasted from chat")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Type != signaltoken.Invitation || inv.Payload.SessionID != "pong-1" {
		t.Fatalf("decoded invitation = %+v", inv)
	}
	if err := redeemer.Redeem(invitation, inv.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	jp, answerSDP, err := rtc.Accept(cfg, inv.Payload.SDP)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := signaltoken.Encode(signaltoken.Answer,
		signaltoken.Payload{SessionID: inv.Payload.SessionID, SDP: answerSDP},
		time.Now().Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	// Host side: decode the answer, finish signaling.
	ans, err := signaltoken.Decode(answer)
	if err != nil {
		t.Fatal(err)
	}
	if ans.Type != signaltoken.Answer || ans.Payload.SessionID != "pong-1" {
		t.Fatalf("decoded answer = %+v", ans)
	}
	if err := hp.Complete(ans.Payload.SDP); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); host, err = hp.Conn(ctx) }()
	var jerr error
	go func() { defer wg.Done(); joiner, jerr = jp.Conn(ctx) }()
	wg.Wait()
	if err != nil || jerr != nil {
		t.Fatalf("conn: host=%v joiner=%v", err, jerr)
	}

	// flow:peer-authentication's binding handle: both sides see the
	// remote DTLS fingerprint a ticket claim would be compared against
	// (rule:ticket-bound-to-connection).
	if hp.Fingerprint() == "" || jp.Fingerprint() == "" {
		t.Fatal("both peers must expose a remote DTLS fingerprint")
	}
	t.Logf("host sees joiner fingerprint %.32s…", hp.Fingerprint())
	t.Logf("joiner sees host fingerprint %.32s…", jp.Fingerprint())
	return host, joiner
}

// The full stack over real WebRTC on loopback, signaled without any
// server (system:webrtc + api:manual-signaling-token): the host runs
// the session, both players join over data channels — ordered reliable
// for control, zero-retransmit for state.
func TestPongOverWebRTC(t *testing.T) {
	redeemer := &signaltoken.Redeemer{}
	serverConns := map[session.SlotID]transport.Conn{}
	clientConns := map[session.SlotID]transport.Conn{}
	for _, slot := range pong.Slots() {
		host, joiner := rtcPair(t, redeemer)
		serverConns[slot], clientConns[slot] = host, joiner
		if !joiner.Capability().UnreliableDatagram {
			t.Fatal("webrtc must offer an unreliable state channel")
		}
		if !joiner.Capability().PeerToPeer {
			t.Fatal("webrtc is the peer-to-peer transport")
		}
	}

	tuning := session.TuningProfile{
		TickRate: 120, SendRate: 60, HistoryDepth: 32, SnapshotEvery: 0,
		BaselineMode: session.BaselineBounded, SpeculationDepth: 8,
		AckMode: 2,
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
			t.Errorf("slot %d: no RTT sample over webrtc", slot)
		}
	}
}
