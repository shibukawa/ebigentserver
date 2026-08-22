// Command tron runs a bot-only light cycle match over the full network
// stack on in-process pipes: admission with self-issued tickets, seq/ack
// datagrams, per-receiver deltas — the same path remote players use, at
// field sizes up to eight.
//
//	tron              # 6 bots, 30Hz
//	tron -bots=8 -duration=20s
package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/budget"
	"github.com/shibukawa/ebigentserver/netplay"
	"github.com/shibukawa/ebigentserver/observe"
	"github.com/shibukawa/ebigentserver/samples/tron/msg"
	"github.com/shibukawa/ebigentserver/samples/tron/tron"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport/pipe"
)

const audience = "local/tron"

func main() {
	bots := flag.Int("bots", 6, "number of cycles (2-8)")
	duration := flag.Duration("duration", 30*time.Second, "maximum match length")
	flag.Parse()
	if *bots < 2 || *bots > tron.MaxPlayers {
		fatal(fmt.Errorf("bots must be 2..%d", tron.MaxPlayers))
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fatal(err)
	}
	verifier := &admission.Verifier{Keys: map[string]ed25519.PublicKey{"k1": pub}, Audience: audience, Leeway: time.Minute}

	var slotIDs []session.SlotID
	for i := 1; i <= *bots; i++ {
		slotIDs = append(slotIDs, session.SlotID(i))
	}
	tuning := session.TuningProfile{
		TickRate: 30, SendRate: 30, HistoryDepth: 16,
		BaselineMode: session.BaselineBounded, SpeculationDepth: 8,
		AckMode: 2, RejectionThreshold: 32, SilenceDeadline: 300,
	}
	bud := budget.Budget{
		MaxSessions: 1, MaxConnections: int32(*bots) + 2, MaxAgents: int32(*bots),
		MaxPendingActions: 64, AdmissionPerSecond: 100, InputsPerTick: 2,
		InputBytesPerSecond: 1 << 16, SendQueueBytes: 1 << 20,
		MaxMessageSize: 1 << 16, MaxPendingReassembly: 8, DrainDeadlineMillis: 1000,
	}
	metrics := &observe.Metrics{}

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	var s *session.Session[tron.State, tron.Input, tron.Observation]
	server, err := netplay.NewServer(ctx, netplay.ServerConfig[tron.State, tron.Input]{
		SessionID: "tron-local", Protocol: msg.SchemaVersion,
		Verifier: verifier, Seed: uint64(time.Now().UnixNano()), Tuning: tuning, Budget: bud,
		MakeSender: func(session.SlotID, string) (statesync.ViewSender[tron.State], error) {
			return statesync.NewSender(tron.Codec(), tuning)
		},
		DecodeInput: func(data []byte) (tron.Input, error) {
			var in msg.TurnInput
			err := in.DecodeCBORFrom(data)
			return in, err
		},
		Inbox:   func(slot session.SlotID) (*session.Inbox[tron.Input], error) { return s.Inbox(slot) },
		Metrics: metrics,
	})
	if err != nil {
		fatal(err)
	}
	s, err = session.New(session.Config[tron.State, tron.Input, tron.Observation]{
		ID: "tron-local", Slots: slotIDs,
		RuleSet:   tron.RuleSet{SlotIDs: slotIDs},
		Validator: tron.Validator{}, Plausibility: tron.Plausibility{FutureWindow: 120},
		Canonical: tron.Canonical, Tuning: &tuning,
		Broadcast: server.Broadcast,
	})
	if err != nil {
		fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		fatal(err)
	}
	for _, slot := range slotIDs {
		if err := s.Admit(slot, session.Detached[tron.Observation, tron.Input]{}); err != nil {
			fatal(err)
		}
	}

	var wg sync.WaitGroup
	var lastState struct {
		mu sync.Mutex
		s  tron.State
	}
	for _, slot := range slotIDs {
		serverConn, clientConn := pipe.Pair(pipe.Faults{}, pipe.Faults{})
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
		ticket, err := admission.Sign(priv, "k1", admission.Claims{
			Subject: fmt.Sprintf("bot-%d", slot), Audience: audience,
			ExpiresAt: time.Now().Add(time.Minute).Unix(),
			ID:        fmt.Sprintf("jti-%d", slot), SessionID: "tron-local",
			Seat: uint16(slot), Role: "player",
		})
		if err != nil {
			fatal(err)
		}
		wg.Add(1)
		go func(slot session.SlotID) {
			defer wg.Done()
			c, err := netplay.Connect(ctx, clientConn, ticket, netplay.ClientConfig[tron.State, tron.Input, msg.TronStateDelta, tron.Observation]{
				Protocol: msg.SchemaVersion, Tuning: tuning, Codec: tron.Codec(),
				EncodeInput: func(dst []byte, a tron.Input) []byte { return a.AppendCBORTo(dst) },
				Project:     func(w *tron.State, sl session.SlotID) tron.Observation { return tron.RuleSet{}.Project(w, sl) },
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, "tron: connect:", err)
				return
			}
			_ = c.Run(ctx, &tron.Bot{})
			if w, _, ok := c.State(); ok {
				lastState.mu.Lock()
				lastState.s = *w
				lastState.mu.Unlock()
			}
		}(slot)
	}

	fmt.Printf("tron: %d bots, %dHz, grid %dx%d\n", *bots, tuning.TickRate, msg.GridW, msg.GridH)
	if err := s.RunRealtime(ctx, session.Paced); err != nil {
		fatal(err)
	}
	cancel()
	wg.Wait()

	lastState.mu.Lock()
	final := lastState.s
	lastState.mu.Unlock()
	if final.Winner != 0 {
		fmt.Printf("winner: slot %d after %d ticks\n", final.Winner, s.Tick())
	} else {
		fmt.Printf("draw after %d ticks\n", s.Tick())
	}
	for _, p := range final.Players {
		if p.Alive {
			fmt.Printf("  slot %d: alive at the end\n", p.ID)
		} else {
			fmt.Printf("  slot %d: crashed at tick %d\n", p.ID, p.DeathTick)
		}
	}
	snap := metrics.Read()
	fmt.Printf("inputs accepted %d, rejected %d, resyncs %d\n",
		snap.InputsAccepted, snap.InputsRejected, snap.ResyncRequests)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tron:", err)
	os.Exit(1)
}
