package tron_test

import (
	"context"
	"crypto/ed25519"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/ebigentserver/admission"
	"github.com/shibukawa/ebigentserver/budget"
	"github.com/shibukawa/ebigentserver/netplay"
	"github.com/shibukawa/ebigentserver/observe"
	"github.com/shibukawa/ebigentserver/samples/tron/msg"
	"github.com/shibukawa/ebigentserver/samples/tron/tron"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/pipe"
	"github.com/shibukawa/ebigentserver/transport/seqack"
)

const netAudience = "sessions.test/tron-1"

func netTicket(t *testing.T, priv ed25519.PrivateKey, jti, role string, seat session.SlotID) string {
	t.Helper()
	tok, err := admission.Sign(priv, "k1", admission.Claims{
		Subject: "p-" + jti, Audience: netAudience,
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		ID:        jti, SessionID: "tron-1", Seat: uint16(seat), Role: role,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func clientConfig(tuning session.TuningProfile) netplay.ClientConfig[tron.State, tron.Input, msg.TronStateDelta, tron.Observation] {
	return netplay.ClientConfig[tron.State, tron.Input, msg.TronStateDelta, tron.Observation]{
		Protocol:    msg.SchemaVersion,
		Tuning:      tuning,
		Codec:       tron.Codec(),
		EncodeInput: func(dst []byte, a tron.Input) []byte { return a.AppendCBORTo(dst) },
		Project:     func(w *tron.State, slot session.SlotID) tron.Observation { return tron.RuleSet{}.Project(w, slot) },
	}
}

// watcher is the spectator's passive agent.
type watcher struct {
	mu   sync.Mutex
	last tron.Observation
}

func (*watcher) Guest(session.SlotID) {}
func (w *watcher) Observe(o tron.Observation) {
	w.mu.Lock()
	w.last = o
	w.mu.Unlock()
}
func (*watcher) Decide(context.Context) (tron.Input, bool) { return tron.Input{}, false }
func (*watcher) Ended(session.Result)                      {}

// Phase 4 acceptance: eight participants plus spectators survive injected
// disconnects, abuse, and overload while requirement:production-runtime-
// safety holds — the session ends normally, honest clients stay synced,
// violators are disconnected with evidence, and the baseline retention
// cost is measured at eight receivers.
func TestEightPlayersSpectatorsAndInjectedFailures(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	verifier := &admission.Verifier{
		Keys: map[string]ed25519.PublicKey{"k1": pub}, Audience: netAudience, Leeway: 5 * time.Second,
	}
	metrics := &observe.Metrics{}
	events := observe.NewLog(4096)
	// Slower ticks than the scripted tests: one cell per tick at 30Hz
	// keeps eight cycles alive long enough for the mid-match injections.
	tuning := testTuning
	tuning.TickRate, tuning.SendRate = 30, 30
	bud := budget.Budget{
		MaxSessions: 1, MaxConnections: 12, MaxAgents: 8, MaxPendingActions: 64,
		AdmissionPerSecond: 100, InputsPerTick: 2, InputBytesPerSecond: 1 << 16,
		SendQueueBytes: 1 << 20, MaxMessageSize: 1 << 16, MaxPendingReassembly: 8,
		DrainDeadlineMillis: 1000,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// The session: 8 seats, plausibility on, escalation declared.
	sim := tron.RuleSet{SlotIDs: slots(8)}
	var s *session.Session[tron.State, tron.Input, tron.Observation]

	takeover := make(chan session.SlotID, 8)
	server, err := netplay.NewServer(ctx, netplay.ServerConfig[tron.State, tron.Input]{
		SessionID: "tron-1", Protocol: msg.SchemaVersion,
		Verifier: verifier, Seed: 9, Tuning: tuning, Budget: bud,
		MakeSender: func(session.SlotID, string) (statesync.ViewSender[tron.State], error) {
			return statesync.NewSender(tron.Codec(), tuning)
		},
		DecodeInput: func(data []byte) (tron.Input, error) {
			var in msg.TurnInput
			err := in.DecodeCBORFrom(data)
			return in, err
		},
		Inbox:   func(slot session.SlotID) (*session.Inbox[tron.Input], error) { return s.Inbox(slot) },
		Metrics: metrics, Events: events,
		OnDeparture: func(slot session.SlotID, role, reason string) {
			// concept:agent-departure-policy: slot 3 gets an AI
			// takeover; every other departure plays on without the
			// seat (continue_without).
			if role != netplay.RoleSpectator && slot == 3 {
				takeover <- slot
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err = session.New(session.Config[tron.State, tron.Input, tron.Observation]{
		ID: "tron-1", Slots: slots(8), RuleSet: sim,
		Validator: tron.Validator{}, Plausibility: tron.Plausibility{FutureWindow: 240},
		Canonical: tron.Canonical, Tuning: &tuning, Seed: 9,
		Broadcast: server.Broadcast,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	for _, slot := range slots(8) {
		if err := s.Admit(slot, session.Detached[tron.Observation, tron.Input]{}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	faults := pipe.Faults{LossPercent: 10, Latency: 8 * time.Millisecond, Jitter: 4 * time.Millisecond, Seed: 5}

	// dial opens a faulty link and runs the server-side admission and
	// peer loop in the background; the returned conn is the client end.
	dial := func(f pipe.Faults) transport.Conn {
		serverConn, clientConn := pipe.Pair(f, f)
		wg.Add(1)
		go func() {
			defer wg.Done()
			peer, err := server.Admit(ctx, serverConn)
			if err != nil {
				return // the handshake reply already carried the reason
			}
			wg.Add(1)
			go peer.Run(ctx, &wg)
		}()
		return clientConn
	}

	// Six honest bots, and slot 3's original human who will vanish.
	clients := map[session.SlotID]*netplay.Client[tron.State, tron.Input, msg.TronStateDelta, tron.Observation]{}
	var clientConns []transport.Conn
	var mu sync.Mutex
	startClient := func(seat session.SlotID, jti string, agent session.Agent[tron.Observation, tron.Input], f pipe.Faults) transport.Conn {
		conn := dial(f)
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := netplay.Connect(ctx, conn, netTicket(t, priv, jti, "player", seat), clientConfig(tuning))
			if err != nil {
				t.Errorf("connect %s: %v", jti, err)
				return
			}
			mu.Lock()
			clients[seat] = c
			clientConns = append(clientConns, conn)
			mu.Unlock()
			_ = c.Run(ctx, agent)
		}()
		return conn
	}

	var doomed3, doomed5 transport.Conn
	for _, seat := range slots(8) {
		f := faults
		f.Seed += uint64(seat)
		switch seat {
		case 7:
			continue // the abuser joins below, outside the client helper
		default:
			conn := startClient(seat, "p"+string(rune('0'+seat)), &tron.Bot{}, f)
			if seat == 3 {
				doomed3 = conn
			}
			if seat == 5 {
				doomed5 = conn
			}
		}
	}

	// The abuser: a validly admitted player who floods garbage.
	abuserConn := dial(pipe.Faults{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := admission.Join(ctx, abuserConn, msg.SchemaVersion, netTicket(t, priv, "p7", "player", 7)); err != nil {
			t.Errorf("abuser join: %v", err)
			return
		}
		layer := seqack.New(abuserConn, seqack.Options{Policy: seqack.PiggybackOnly})
		for i := 0; ctx.Err() == nil && i < 500; i++ {
			_ = layer.SendDatagram(ctx, []byte{0xFF, 0xFE, byte(i)}, 0) // malformed input payloads
			time.Sleep(2 * time.Millisecond)
		}
	}()

	// Two spectators: one honest, one cheating (submits inputs).
	specConn := dial(faults)
	honestSpec := &watcher{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := netplay.Connect(ctx, specConn, netTicket(t, priv, "spec1", netplay.RoleSpectator, 0), clientConfig(tuning))
		if err != nil {
			t.Errorf("spectator connect: %v", err)
			return
		}
		_ = c.Run(ctx, honestSpec)
	}()
	cheatConn := dial(pipe.Faults{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := admission.Join(ctx, cheatConn, msg.SchemaVersion, netTicket(t, priv, "spec2", netplay.RoleSpectator, 0)); err != nil {
			return
		}
		layer := seqack.New(cheatConn, seqack.Options{Policy: seqack.PiggybackOnly})
		in := tron.Input{Dir: tron.DirLeft}
		for i := 0; ctx.Err() == nil && i < 200; i++ {
			_ = layer.SendDatagram(ctx, in.AppendCBORTo(nil), 0) // receive-only role submitting actions
			time.Sleep(3 * time.Millisecond)
		}
	}()

	// Departure injections while the match runs.
	time.AfterFunc(400*time.Millisecond, func() { doomed3.Close() }) // → ai_takeover
	time.AfterFunc(700*time.Millisecond, func() { doomed5.Close() }) // → continue_without

	// The takeover handler: a fresh admission seats a bot on slot 3 —
	// human and bot are the same agent, so takeover is just another
	// client (decision:agent-as-central-abstraction).
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case seat := <-takeover:
			startClient(seat, "takeover3", &tron.Bot{}, pipe.Faults{})
		case <-ctx.Done():
		}
	}()

	// Baseline retention cost at eight-plus receivers, sampled while the
	// match runs and every surviving peer stands
	// (decision:framework-side-delta-generation's cost note).
	type retention struct{ receivers, versions int }
	measured := make(chan retention, 1)
	time.AfterFunc(3*time.Second, func() {
		r, v := server.RetainedCost()
		measured <- retention{r, int(v)}
	})

	// Run the match.
	if err := s.RunRealtime(ctx, session.Paced); err != nil {
		t.Fatal(err)
	}
	finalTick := s.Tick()
	ret := <-measured
	receivers, versions, bytesPer := ret.receivers, int32(ret.versions), 0
	cancel()
	mu.Lock()
	for _, c := range clientConns {
		c.Close()
	}
	mu.Unlock()
	abuserConn.Close()
	specConn.Close()
	cheatConn.Close()
	wg.Wait()

	if s.State() != session.StateEnded {
		t.Fatalf("session state = %v, want ended", s.State())
	}
	if finalTick < 60 {
		t.Fatalf("only %d ticks committed; metrics %+v; events %v", finalTick, metrics.Read(), events.Events()[:min(len(events.Events()), 12)])
	}

	// Honest clients stayed synced under loss and departures.
	mu.Lock()
	defer mu.Unlock()
	synced := 0
	for seat, c := range clients {
		if _, tick, ok := c.State(); ok && tick >= finalTick*7/10 {
			synced++
		} else {
			t.Logf("seat %d: not fully synced (departed on purpose?)", seat)
		}
	}
	if synced < 5 {
		t.Errorf("only %d honest clients stayed synced", synced)
	}
	// The honest spectator watched the whole thing without sending a byte.
	honestSpec.mu.Lock()
	specTick := honestSpec.last.State.Tick
	honestSpec.mu.Unlock()
	if specTick < uint64(finalTick)*7/10 {
		t.Errorf("spectator reconstructed only to tick %d of %d", specTick, finalTick)
	}

	// Evidence (api:runtime-observability): departures, abuse, capacity.
	if n := events.CountKind("departure"); n < 2 {
		t.Errorf("departure events = %d, want >= 2", n)
	}
	if n := events.CountKind("abuse_reject"); n == 0 {
		t.Error("no abuse_reject evidence for the flood and the cheating spectator")
	}
	if n := events.CountKind("disconnect"); n == 0 {
		t.Error("no disconnect evidence for the abuse threshold")
	}
	snap := metrics.Read()
	if snap.InputsAccepted == 0 || snap.InputsRejected == 0 || snap.Disconnects == 0 {
		t.Errorf("metrics incomplete: %+v", snap)
	}

	// Report the retention measurement using a synced client's final
	// state as the size sample.
	if c, ok := clients[1]; ok {
		if w, _, ok := c.State(); ok {
			bytesPer = len(tron.Canonical(w))
		}
	}
	t.Logf("baseline retention: %d receivers x %d versions x ~%d bytes = ~%d KiB total",
		receivers, versions, bytesPer, receivers*int(versions)*bytesPer/1024)
	if receivers < 6 {
		var trail []observe.Event
		for _, ev := range events.Events() {
			if ev.Kind == "departure" || ev.Kind == "disconnect" {
				trail = append(trail, ev)
			}
		}
		t.Errorf("only %d receivers stood at match end; departures/disconnects: %+v", receivers, trail)
	}
	if versions != tuning.HistoryDepth {
		t.Errorf("retained versions %d != declared history depth %d", versions, tuning.HistoryDepth)
	}
}

// Admission fails closed at the declared connection ceiling, before any
// allocation (policy:realtime-abuse-protection, data:runtime-resource-
// budget).
func TestAdmissionCapacityFailsClosed(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	metrics := &observe.Metrics{}
	events := observe.NewLog(16)
	bud := budget.Budget{
		MaxSessions: 1, MaxConnections: 1, MaxAgents: 1, MaxPendingActions: 8,
		AdmissionPerSecond: 10, InputsPerTick: 2, InputBytesPerSecond: 1 << 12,
		SendQueueBytes: 1 << 16, MaxMessageSize: 1 << 12, MaxPendingReassembly: 2,
		DrainDeadlineMillis: 100,
	}
	ctx := context.Background()
	server, err := netplay.NewServer(ctx, netplay.ServerConfig[tron.State, tron.Input]{
		SessionID: "cap", Protocol: msg.SchemaVersion,
		Verifier: &admission.Verifier{Keys: map[string]ed25519.PublicKey{"k1": pub}, Audience: netAudience},
		Tuning:   testTuning, Budget: bud,
		MakeSender: func(session.SlotID, string) (statesync.ViewSender[tron.State], error) {
			return statesync.NewSender(tron.Codec(), testTuning)
		},
		DecodeInput: func(data []byte) (tron.Input, error) {
			var in msg.TurnInput
			err := in.DecodeCBORFrom(data)
			return in, err
		},
		Inbox: func(session.SlotID) (*session.Inbox[tron.Input], error) {
			return nil, context.Canceled // never reached in this test
		},
		Metrics: metrics, Events: events,
	})
	if err != nil {
		t.Fatal(err)
	}
	// First connection occupies the only slot (admission runs async and
	// blocks on the missing Hello; the count is taken synchronously).
	sc1, cc1 := pipe.Pair(pipe.Faults{}, pipe.Faults{})
	defer cc1.Close()
	go func() { _, _ = server.Admit(ctx, sc1) }()
	time.Sleep(50 * time.Millisecond)
	sc2, cc2 := pipe.Pair(pipe.Faults{}, pipe.Faults{})
	defer cc2.Close()
	if _, err := server.Admit(ctx, sc2); err == nil {
		t.Fatal("admission beyond MaxConnections must fail")
	}
	if metrics.Read().AdmissionRejected == 0 || events.CountKind("admission_reject") == 0 {
		t.Error("capacity rejection left no evidence")
	}
	sc1.Close()
}
