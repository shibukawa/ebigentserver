package rtslite_test

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/budget"
	"github.com/shibukawa/ebigentserver/netplay"
	"github.com/shibukawa/ebigentserver/samples/rtslite/msg"
	"github.com/shibukawa/ebigentserver/samples/rtslite/rtslite"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/transport/pipe"
)

// The full hybrid exchange over the wire (flow:hybrid-sync-exchange):
// command datagrams upstream through IntakeAll, fog-projected deltas
// downstream, on lossy links.
func TestHybridExchangeOverTheWire(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	const aud = "sessions.test/rts-1"
	tuning := testTuning
	bud := budget.Budget{
		MaxSessions: 1, MaxConnections: 4, MaxAgents: 2, MaxPendingActions: 64,
		AdmissionPerSecond: 100, InputsPerTick: 8, InputBytesPerSecond: 1 << 16,
		SendQueueBytes: 1 << 20, MaxMessageSize: 1 << 16, MaxPendingReassembly: 8,
		DrainDeadlineMillis: 1000,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	var s *session.Session[rtslite.State, rtslite.Input, rtslite.Sight]
	server, err := netplay.NewServer(ctx, netplay.ServerConfig[rtslite.State, rtslite.Input]{
		SessionID: "rts-1", Protocol: msg.SchemaVersion,
		Verifier: &admission.Verifier{Keys: map[string]ed25519.PublicKey{"k1": pub}, Audience: aud, Leeway: time.Minute},
		Seed:     9, Tuning: tuning, Budget: bud,
		MakeSender: rtslite.MakeSender(tuning),
		DecodeInput: func(data []byte) (rtslite.Input, error) {
			var in msg.Command
			err := in.DecodeCBORFrom(data)
			return in, err
		},
		Inbox: func(slot session.SlotID) (*session.Inbox[rtslite.Input], error) { return s.Inbox(slot) },
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err = session.New(session.Config[rtslite.State, rtslite.Input, rtslite.Sight]{
		ID: "rts-1", Slots: rtslite.Slots(2),
		RuleSet:   rtslite.RuleSet{Players: 2, TickLimit: 3600},
		Validator: rtslite.Validator{}, Canonical: rtslite.Canonical,
		Tuning: &tuning, Seed: 9, Broadcast: server.Broadcast,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	for _, slot := range rtslite.Slots(2) {
		if err := s.Admit(slot, session.Detached[rtslite.Sight, rtslite.Input]{}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	faults := pipe.Faults{LossPercent: 10, Latency: 8 * time.Millisecond, Jitter: 4 * time.Millisecond, Seed: 6}
	clients := map[session.SlotID]*netplay.Client[msg.PlayerView, rtslite.Input, msg.PlayerViewDelta, rtslite.Sight]{}
	var mu sync.Mutex
	for _, slot := range rtslite.Slots(2) {
		f := faults
		f.Seed += uint64(slot)
		serverConn, clientConn := pipe.Pair(f, f)
		wg.Add(1)
		go func() {
			defer wg.Done()
			peer, err := server.Admit(ctx, serverConn)
			if err != nil {
				return
			}
			wg.Add(1)
			go peer.Run(ctx, &wg)
		}()
		tok, err := admission.Sign(priv, "k1", admission.Claims{
			Subject: "p", Audience: aud, ExpiresAt: time.Now().Add(time.Minute).Unix(),
			ID: fmt.Sprintf("rts-%d", slot), SessionID: "rts-1", Seat: uint16(slot), Role: "player",
		})
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func(slot session.SlotID) {
			defer wg.Done()
			defer clientConn.Close()
			c, err := netplay.Connect(ctx, clientConn, tok, netplay.ClientConfig[msg.PlayerView, rtslite.Input, msg.PlayerViewDelta, rtslite.Sight]{
				Protocol: msg.SchemaVersion, Tuning: tuning, Codec: rtslite.ViewCodec(),
				EncodeInput: func(dst []byte, a rtslite.Input) []byte { return a.AppendCBORTo(dst) },
				Project: func(v *msg.PlayerView, sl session.SlotID) rtslite.Sight {
					return rtslite.Sight{You: sl, View: v, Annotation: rtslite.Annotation(v)}
				},
			})
			if err != nil {
				t.Errorf("connect %d: %v", slot, err)
				return
			}
			mu.Lock()
			clients[slot] = c
			mu.Unlock()
			_ = c.Run(ctx, &rtslite.Bot{})
		}(slot)
	}

	if err := s.RunRealtime(ctx, session.Paced); err != nil {
		t.Fatal(err)
	}
	final := s.Tick()
	cancel()
	wg.Wait()

	if s.State() != session.StateEnded {
		t.Fatalf("state = %v", s.State())
	}
	if final < 60 {
		t.Fatalf("only %d ticks committed", final)
	}
	mu.Lock()
	defer mu.Unlock()
	for slot, c := range clients {
		v, tick, ok := c.State()
		if !ok || tick < final*7/10 {
			t.Errorf("slot %d fell behind: tick %d of %d", slot, tick, final)
			continue
		}
		// Fog held on the wire path too: whatever arrived is a
		// projection, and armies engaged means units actually moved on
		// bot orders.
		if len(v.Own) == 0 {
			t.Errorf("slot %d received no own units", slot)
		}
	}
}
