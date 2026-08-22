package dungeon_test

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
	"github.com/shibukawa/ebigentserver/samples/dungeon/dungeon"
	"github.com/shibukawa/ebigentserver/samples/dungeon/msg"
	"github.com/shibukawa/ebigentserver/session"
	"github.com/shibukawa/ebigentserver/statesync"
	"github.com/shibukawa/ebigentserver/transport"
	"github.com/shibukawa/ebigentserver/transport/pipe"
	"github.com/shibukawa/ebigentserver/transport/seqack"
)

const dgAudience = "sessions.test/dungeon-1"

var dgTuning = session.TuningProfile{
	TickRate: 30, SendRate: 30, HistoryDepth: 8, SnapshotEvery: 0,
	BaselineMode: session.BaselineBounded, SpeculationDepth: 4,
	AckMode: 2, RejectionThreshold: 32, SilenceDeadline: 300,
}

var dgBudget = budget.Budget{
	MaxSessions: 1, MaxConnections: 8, MaxAgents: 5, MaxPendingActions: 64,
	AdmissionPerSecond: 100, InputsPerTick: 2, InputBytesPerSecond: 1 << 16,
	SendQueueBytes: 1 << 20, MaxMessageSize: 1 << 16, MaxPendingReassembly: 8,
	DrainDeadlineMillis: 1000,
}

// recordingConn tees every received unreliable payload: the captured
// bytes are literally everything the server ever sent this client.
type recordingConn struct {
	transport.Conn
	mu       sync.Mutex
	payloads [][]byte
}

func (r *recordingConn) Receive(ctx context.Context) (transport.Message, error) {
	m, err := r.Conn.Receive(ctx)
	if err == nil && m.Channel == transport.Unreliable {
		c := append([]byte(nil), m.Payload...)
		r.mu.Lock()
		r.payloads = append(r.payloads, c)
		r.mu.Unlock()
	}
	return m, err
}

// script feeds a fixed move list through the same Agent interface a human
// client uses.
type script struct {
	moves []dungeon.Input
	last  Observation
}

type Observation = dungeon.Observation

func (*script) Joined(session.SlotID)   {}
func (s *script) Observe(o Observation) { s.last = o }
func (*script) Ended(session.Result)    {}
func (s *script) Decide(context.Context) (dungeon.Input, bool) {
	if len(s.moves) == 0 {
		return dungeon.Input{}, false
	}
	m := s.moves[0]
	s.moves = s.moves[1:]
	return m, true
}

type dungeonNet struct {
	t       *testing.T
	priv    ed25519.PrivateKey
	server  *netplay.Server[dungeon.State, dungeon.Input]
	sess    *session.Session[dungeon.State, dungeon.Input, Observation]
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	jti     int
	tickLim uint32
}

func newDungeonNet(t *testing.T, adventurers int, tickLimit uint32, d time.Duration) *dungeonNet {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	n := &dungeonNet{t: t, priv: priv, ctx: ctx, cancel: cancel, tickLim: tickLimit}

	var s *session.Session[dungeon.State, dungeon.Input, Observation]
	server, err := netplay.NewServer(ctx, netplay.ServerConfig[dungeon.State, dungeon.Input]{
		SessionID: "dungeon-1", Protocol: msg.CBORProtocolVersion,
		Verifier: &admission.Verifier{Keys: map[string]ed25519.PublicKey{"k1": pub}, Audience: dgAudience, Leeway: time.Minute},
		Seed:     77, Tuning: dgTuning, Budget: dgBudget,
		MakeSender: dungeon.MakeSender(dgTuning),
		DecodeInput: func(data []byte) (dungeon.Input, error) {
			var in msg.ActionInput
			err := in.DecodeCBORFrom(data)
			return in, err
		},
		Inbox: func(slot session.SlotID) (*session.Inbox[dungeon.Input], error) { return s.Inbox(slot) },
	})
	if err != nil {
		t.Fatal(err)
	}
	sim := dungeon.Simulation{Adventurers: adventurers, TickLimit: tickLimit}
	s, err = session.New(session.Config[dungeon.State, dungeon.Input, Observation]{
		ID: "dungeon-1", Slots: dungeon.Slots(adventurers), Simulation: sim,
		Validator: dungeon.Validator{}, Canonical: dungeon.Canonical,
		Tuning: &dgTuning, Seed: 77, Broadcast: server.Broadcast,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.OpenAdmission(); err != nil {
		t.Fatal(err)
	}
	for _, slot := range dungeon.Slots(adventurers) {
		if err := s.Admit(slot, session.Detached[Observation, dungeon.Input]{}); err != nil {
			t.Fatal(err)
		}
	}
	n.server, n.sess = server, s
	return n
}

func (n *dungeonNet) ticket(seat session.SlotID) string {
	n.jti++
	tok, err := admission.Sign(n.priv, "k1", admission.Claims{
		Subject: "p", Audience: dgAudience,
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
		ID:        fmt.Sprintf("%s-%d", n.t.Name(), n.jti),
		SessionID: "dungeon-1", Seat: uint16(seat), Role: "player",
	})
	if err != nil {
		n.t.Fatal(err)
	}
	return tok
}

// connect wires one participant; conn may be pre-wrapped for recording.
func (n *dungeonNet) serveSide(conn transport.Conn) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		peer, err := n.server.Admit(n.ctx, conn)
		if err != nil {
			return
		}
		n.wg.Add(1)
		go peer.Run(n.ctx, &n.wg)
	}()
}

// dmClientCfg and advClientCfg differ in their view types — the two
// sides of the asymmetry.
func dmClientCfg() netplay.ClientConfig[msg.DMView, dungeon.Input, msg.DMViewDelta, Observation] {
	return netplay.ClientConfig[msg.DMView, dungeon.Input, msg.DMViewDelta, Observation]{
		Protocol: msg.CBORProtocolVersion, Tuning: dgTuning, Codec: dungeon.DMCodec(),
		EncodeInput: func(dst []byte, a dungeon.Input) []byte { return a.AppendCBORTo(dst) },
		Project: func(v *msg.DMView, slot session.SlotID) Observation {
			return Observation{You: slot, Role: msg.RoleDM, DM: v, Annotation: dungeon.DMAnnotation(v)}
		},
	}
}

func advClientCfg() netplay.ClientConfig[msg.AdventurerView, dungeon.Input, msg.AdventurerViewDelta, Observation] {
	return netplay.ClientConfig[msg.AdventurerView, dungeon.Input, msg.AdventurerViewDelta, Observation]{
		Protocol: msg.CBORProtocolVersion, Tuning: dgTuning, Codec: dungeon.AdventurerCodec(),
		EncodeInput: func(dst []byte, a dungeon.Input) []byte { return a.AppendCBORTo(dst) },
		Project: func(v *msg.AdventurerView, slot session.SlotID) Observation {
			return Observation{You: slot, Role: v.Role, Adventurer: v, Annotation: dungeon.AdventurerAnnotation(v)}
		},
	}
}

// Phase 5 acceptance, criterion 2: hidden information is not transmitted
// — not display-filtered, not sent. Every byte the scout's client ever
// received is captured and re-decoded; no undiscovered trap, no wall
// outside explored cells, and no exit position exists anywhere in it,
// while the DM demonstrably placed traps the whole time.
func TestHiddenInfoNeverOnTheWire(t *testing.T) {
	n := newDungeonNet(t, 2, 240, 6*time.Second) // scout + engineer, no navigator
	defer n.cancel()

	// DM: a bot that keeps placing traps.
	dmConn, dmClient := pipe.Pair(pipe.Faults{}, pipe.Faults{})
	n.serveSide(dmConn)
	var dmTraps int
	dmTok := n.ticket(dungeon.SlotDM)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		c, err := netplay.Connect(n.ctx, dmClient, dmTok, dmClientCfg())
		if err != nil {
			t.Errorf("dm connect: %v", err)
			return
		}
		_ = c.Run(n.ctx, &dungeon.DMBot{})
		if v, _, ok := c.State(); ok {
			dmTraps = len(v.Traps)
		}
	}()

	// Scout: recorded link.
	scoutServer, scoutClientRaw := pipe.Pair(pipe.Faults{}, pipe.Faults{})
	scoutConn := &recordingConn{Conn: scoutClientRaw}
	n.serveSide(scoutServer)
	scoutTok := n.ticket(2)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		c, err := netplay.Connect(n.ctx, scoutConn, scoutTok, advClientCfg())
		if err != nil {
			t.Errorf("scout connect: %v", err)
			return
		}
		_ = c.Run(n.ctx, &dungeon.AdventurerBot{})
	}()
	// Engineer keeps the party moving.
	engServer, engClient := pipe.Pair(pipe.Faults{}, pipe.Faults{})
	n.serveSide(engServer)
	engTok := n.ticket(3)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		c, err := netplay.Connect(n.ctx, engClient, engTok, advClientCfg())
		if err != nil {
			return
		}
		_ = c.Run(n.ctx, &dungeon.AdventurerBot{})
	}()

	if err := n.sess.RunRealtime(n.ctx, session.Paced); err != nil {
		t.Fatal(err)
	}
	n.cancel()
	dmClient.Close()
	scoutClientRaw.Close()
	engClient.Close()
	n.wg.Wait()

	// Re-decode every captured byte through a fresh receive chain.
	scoutConn.mu.Lock()
	payloads := scoutConn.payloads
	scoutConn.mu.Unlock()
	if len(payloads) < 10 {
		t.Fatalf("only %d payloads captured", len(payloads))
	}
	layer := seqack.New(nil, seqack.Options{})
	recv, err := statesync.NewReceiver(dungeon.AdventurerCodec(), dgTuning)
	if err != nil {
		t.Fatal(err)
	}
	views := 0
	for _, raw := range payloads {
		data := layer.Absorb(raw)
		if data == nil {
			continue
		}
		pkt, err := statesync.DecodeWire(data)
		if err != nil {
			continue
		}
		if err := recv.Apply(pkt); err != nil {
			continue // gaps from the capture point are fine here
		}
		v, _, ok := recv.State()
		if !ok {
			continue
		}
		views++
		assertAdventurerViewInvariants(t, v, false)
	}
	if views < 10 {
		t.Fatalf("only %d views reconstructed from the capture", views)
	}
	if dmTraps == 0 {
		t.Fatal("the DM never placed a trap; the hidden-info claim proved nothing")
	}
	t.Logf("captured %d payloads, %d views, %d traps existed server-side", len(payloads), views, dmTraps)
}

// Phase 5 acceptance, criteria 1 and 3: the DM and the party receive
// views that differ in kind, and every controller combination (human or
// AI DM against human or AI party) completes a session.
func TestAllFourControllerCombos(t *testing.T) {
	combos := []struct {
		name              string
		dmHuman, advHuman bool
	}{
		{"ai-dm_vs_ai-party", false, false},
		{"ai-dm_vs_human-party", false, true},
		{"human-dm_vs_ai-party", true, false},
		{"human-dm_vs_human-party", true, true},
	}
	for _, combo := range combos {
		t.Run(combo.name, func(t *testing.T) {
			n := newDungeonNet(t, 1, 75, 8*time.Second) // short crawl; DM wins by timeout unless the party escapes
			defer n.cancel()

			var dmAgent session.Agent[Observation, dungeon.Input] = &dungeon.DMBot{}
			if combo.dmHuman {
				dmAgent = &script{moves: []dungeon.Input{
					{Kind: msg.ActPlaceTrap, X: msg.GridW - 4, Y: msg.GridH - 4},
					{Kind: msg.ActPlaceTrap, X: msg.GridW - 6, Y: msg.GridH - 6},
				}}
			}
			var advAgent session.Agent[Observation, dungeon.Input] = &dungeon.AdventurerBot{}
			if combo.advHuman {
				moves := make([]dungeon.Input, 0, 60)
				for i := 0; i < 30; i++ {
					moves = append(moves, dungeon.Input{Kind: msg.ActMove, Dir: 1}, dungeon.Input{Kind: msg.ActMove, Dir: 2})
				}
				advAgent = &script{moves: moves}
			}

			var dmKind, advKind bool
			var mu sync.Mutex
			run := func(seat session.SlotID, agent session.Agent[Observation, dungeon.Input]) {
				serverConn, clientConn := pipe.Pair(pipe.Faults{}, pipe.Faults{})
				n.serveSide(serverConn)
				tok := n.ticket(seat) // minted before the goroutine: jti issuance is not concurrent-safe
				n.wg.Add(1)
				go func() {
					defer n.wg.Done()
					defer clientConn.Close()
					if seat == dungeon.SlotDM {
						c, err := netplay.Connect(n.ctx, clientConn, tok, dmClientCfg())
						if err != nil {
							t.Errorf("dm connect: %v", err)
							return
						}
						_ = c.Run(n.ctx, agent)
						if v, _, ok := c.State(); ok && v.Walls != nil {
							mu.Lock()
							dmKind = true // the DM view carries the whole map
							mu.Unlock()
						}
					} else {
						c, err := netplay.Connect(n.ctx, clientConn, tok, advClientCfg())
						if err != nil {
							t.Errorf("adventurer connect: %v", err)
							return
						}
						_ = c.Run(n.ctx, agent)
						if v, _, ok := c.State(); ok && v.ExitX == msg.Unknown {
							mu.Lock()
							advKind = true // the scout view does not know the exit
							mu.Unlock()
						}
					}
				}()
			}
			run(dungeon.SlotDM, dmAgent)
			run(2, advAgent)

			if err := n.sess.RunRealtime(n.ctx, session.Paced); err != nil {
				t.Fatal(err)
			}
			n.cancel()
			n.wg.Wait()

			if n.sess.State() != session.StateEnded {
				t.Fatalf("state = %v", n.sess.State())
			}
			mu.Lock()
			defer mu.Unlock()
			if !dmKind || !advKind {
				t.Fatalf("view kinds not both observed: dm=%v adv=%v", dmKind, advKind)
			}
		})
	}
}
