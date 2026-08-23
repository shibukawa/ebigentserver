package rtslite_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/shibukawa/ebigentserver/episode"
	"github.com/shibukawa/ebigentserver/samples/rtslite/msg"
	"github.com/shibukawa/ebigentserver/samples/rtslite/rtslite"
	"github.com/shibukawa/ebigentserver/session"
)

var testTuning = session.TuningProfile{
	TickRate: 30, SendRate: 15, HistoryDepth: 16, SnapshotEvery: 0,
	BaselineMode: session.BaselineBounded, SpeculationDepth: 8,
	InputIntake: session.IntakeAll, AckMode: 2,
}

func TestOrdersAndOwnership(t *testing.T) {
	g := rtslite.RuleSet{Players: 2, TickLimit: 600}
	s := g.Start(3)
	v := rtslite.Validator{}
	mine := s.Units[0].ID                        // owner 1
	theirs := s.Units[rtslite.UnitsPerPlayer].ID // owner 2

	if msg.OwnerOf(mine) != 1 || msg.OwnerOf(theirs) != 2 {
		t.Fatalf("owner packing broken: %d %d", msg.OwnerOf(mine), msg.OwnerOf(theirs))
	}
	// You command your own units only.
	if err := v.Legal(&s, 1, rtslite.Input{Unit: theirs, TargetX: 10, TargetY: 10}); err == nil {
		t.Error("ordered an enemy unit")
	}
	if err := v.Legal(&s, 1, rtslite.Input{Unit: mine, TargetX: 200, TargetY: 10}); err == nil {
		t.Error("target off the map accepted")
	}
	order := rtslite.Input{Unit: mine, TargetX: 20, TargetY: 20}
	if err := v.Legal(&s, 1, order); err != nil {
		t.Fatalf("legal order rejected: %v", err)
	}
	// The unit walks one step per tick toward the standing target.
	g.Apply(&s, 1, order)
	x0, y0 := s.Units[0].X, s.Units[0].Y
	g.Advance(&s)
	if s.Units[0].X == x0 && s.Units[0].Y == y0 {
		t.Fatal("unit did not move")
	}
}

func TestCombatEndsTheGame(t *testing.T) {
	g := rtslite.RuleSet{Players: 2, TickLimit: 4000}
	s := g.Start(3)
	// March both armies onto the same cell block until one side dies.
	for slot := session.SlotID(1); slot <= 2; slot++ {
		for _, u := range s.Units {
			if msg.OwnerOf(u.ID) == uint16(slot) {
				g.Apply(&s, slot, rtslite.Input{Unit: u.ID, TargetX: msg.MapW / 2, TargetY: msg.MapH / 2})
			}
		}
	}
	for i := 0; i < 4000 && !s.Over; i++ {
		g.Advance(&s)
	}
	if !s.Over {
		t.Fatal("battle never resolved")
	}
	// Someone won or mutual annihilation drew; both are legal ends.
	t.Logf("winner=%d after %d ticks", s.Winner, s.Tick)
}

// term:fog-of-war as a projection predicate: enemies exist for a player
// only inside sight of an own unit.
func TestFogOfWar(t *testing.T) {
	g := rtslite.RuleSet{Players: 2, TickLimit: 600}
	s := g.Start(3)

	// Armies start in opposite corners, far outside sight range.
	v1 := rtslite.ProjectPlayer(&s, 1)
	if len(v1.Visible) != 0 {
		t.Fatalf("enemies visible across the map: %d", len(v1.Visible))
	}
	if int(v1.OwnAlive) != rtslite.UnitsPerPlayer || len(v1.Own) != rtslite.UnitsPerPlayer {
		t.Fatalf("own army projection: %d/%d", v1.OwnAlive, len(v1.Own))
	}
	// Announced but unlocated: total enemy count is known.
	if int(v1.EnemyAlive) != rtslite.UnitsPerPlayer {
		t.Fatalf("enemy count: %d", v1.EnemyAlive)
	}
	// Teleport one enemy next to player 1's first unit: exactly the
	// intruder becomes visible.
	i := rtslite.UnitsPerPlayer // first unit of player 2
	s.Units[i].X, s.Units[i].Y = s.Units[0].X+2, s.Units[0].Y
	v1 = rtslite.ProjectPlayer(&s, 1)
	if len(v1.Visible) != 1 || v1.Visible[0].ID != s.Units[i].ID {
		t.Fatalf("visible = %+v, want just the intruder", v1.Visible)
	}
	// And symmetric fog: player 2 sees player 1's units near its scout.
	v2 := rtslite.ProjectPlayer(&s, 2)
	if len(v2.Visible) == 0 {
		t.Fatal("player 2 blind next to the enemy army")
	}
}

// The two CBOR profiles side by side: a command is a handful of bytes on
// the wire profile; a fog view snapshot is a large world-profile map.
func TestBothProfilesInOneGame(t *testing.T) {
	cmd := rtslite.Input{Tick: 1234, Unit: msg.MakeUnitID(1, 7), TargetX: 40, TargetY: 30}
	wire := cmd.AppendCBORTo(nil)
	if len(wire) > 16 {
		t.Fatalf("wire-profile command is %d bytes", len(wire))
	}
	g := rtslite.RuleSet{Players: 4, TickLimit: 600}
	s := g.Start(3)
	view := rtslite.ProjectPlayer(&s, 1)
	snap := view.AppendCBORTo(nil)
	if len(snap) < 256 {
		t.Fatalf("world-profile view snapshot is only %d bytes", len(snap))
	}
	t.Logf("command %dB (wire profile) vs view snapshot %dB (world profile)", len(wire), len(snap))
}

// scriptedCommands drives a deterministic 2-player battle: each slot
// re-targets a few units every tick, several orders per tick — the
// IntakeAll command stream.
func scriptedCommands(s0 *rtslite.State) func(tick session.Tick, slot session.SlotID) (rtslite.Input, bool) {
	perTick := map[uint64]map[session.SlotID][]rtslite.Input{}
	// Precompute: at each 8th tick, both players order 4 units toward
	// the center, staggered by unit index.
	var ids [3][]uint32
	for _, u := range s0.Units {
		o := msg.OwnerOf(u.ID)
		ids[o] = append(ids[o], u.ID)
	}
	for tick := uint64(0); tick < 400; tick += 8 {
		m := map[session.SlotID][]rtslite.Input{}
		for slot := session.SlotID(1); slot <= 2; slot++ {
			for k := 0; k < 4; k++ {
				idx := (int(tick/8)*4 + k) % len(ids[slot])
				m[slot] = append(m[slot], rtslite.Input{
					Tick: uint32(tick), Unit: ids[slot][idx],
					TargetX: msg.MapW/2 + uint8(slot), TargetY: msg.MapH/2 + uint8(k),
				})
			}
		}
		perTick[tick] = m
	}
	return func(tick session.Tick, slot session.SlotID) (rtslite.Input, bool) {
		q := perTick[uint64(tick)][slot]
		if len(q) == 0 {
			return rtslite.Input{}, false
		}
		perTick[uint64(tick)][slot] = q[1:]
		return q[0], true
	}
}

type logs struct{ decisions, events, outcomes, world bytes.Buffer }

func record(t *testing.T, src func(session.Tick, session.SlotID) (rtslite.Input, bool)) (*logs, session.Tick) {
	t.Helper()
	var l logs
	w := episode.NewWriter[rtslite.State, rtslite.Input, rtslite.Sight](
		episode.Streams{Decisions: &l.decisions, Events: &l.events, Outcomes: &l.outcomes, World: &l.world},
		episode.ReplayComplete,
		episode.Meta{EpisodeID: "rts-ep", ProtocolVersion: msg.SchemaVersion},
	)
	g := rtslite.RuleSet{Players: 2, TickLimit: 400}
	tuning := testTuning
	s, err := session.New(session.Config[rtslite.State, rtslite.Input, rtslite.Sight]{
		ID: "rts-test", Slots: rtslite.Slots(2), RuleSet: g,
		Validator: rtslite.Validator{}, Canonical: rtslite.Canonical,
		Tuning: &tuning, Clock: func() int64 { return 0 },
		Seed: 3, Recorder: w, InputSource: src,
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
	if err := s.RunRealtime(context.Background(), session.Unlimited); err != nil {
		t.Fatal(err)
	}
	if err := w.Err(); err != nil {
		t.Fatal(err)
	}
	return &l, s.Tick()
}

// Multi-command ticks record and replay bit-identically: the schedule
// reader hands back each slot's several orders in recorded FIFO order.
func TestCommandStreamRecordReplaysBitIdentical(t *testing.T) {
	g := rtslite.RuleSet{Players: 2, TickLimit: 400}
	s0 := g.Start(3)
	original, _ := record(t, scriptedCommands(&s0))
	_, schedule, err := episode.ReadReplaySchedule[rtslite.Input](bytes.NewReader(original.decisions.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	replayed, _ := record(t, schedule)
	for _, cmp := range []struct {
		name          string
		first, second *bytes.Buffer
	}{
		{"decisions", &original.decisions, &replayed.decisions},
		{"outcomes", &original.outcomes, &replayed.outcomes},
		{"world", &original.world, &replayed.world},
	} {
		if !bytes.Equal(cmp.first.Bytes(), cmp.second.Bytes()) {
			t.Errorf("%s stream differs between original and replay", cmp.name)
		}
	}
	// The events stream legitimately differs in one way: the original
	// run records validator rejections (orders for units that died in
	// the meantime), and a replay of accepted actions has none to
	// reject. The determinism proof is the checkpoint chain, which must
	// match exactly.
	cp1, err := episode.ReadCheckpoints(bytes.NewReader(original.events.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	cp2, err := episode.ReadCheckpoints(bytes.NewReader(replayed.events.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(cp1) == 0 || len(cp1) != len(cp2) {
		t.Fatalf("checkpoint counts differ: %d vs %d", len(cp1), len(cp2))
	}
	for i := range cp1 {
		if cp1[i] != cp2[i] {
			t.Fatalf("checkpoint %d differs: %+v vs %+v", i, cp1[i], cp2[i])
		}
	}
}

// The scripted battle's final checkpoint pins across architectures.
func TestScriptedBattleDigestPinned(t *testing.T) {
	g := rtslite.RuleSet{Players: 2, TickLimit: 400}
	s0 := g.Start(3)
	l, ticks := record(t, scriptedCommands(&s0))
	cps, err := episode.ReadCheckpoints(bytes.NewReader(l.events.Bytes()))
	if err != nil || len(cps) == 0 {
		t.Fatalf("checkpoints: %v (%d)", err, len(cps))
	}
	last := cps[len(cps)-1]
	// The world digests moved once, when concept:cbor-world-profile became the
	// map shape of tinybind v0.5.23: the profile's integer labels are gone and
	// members carry their names. The action digests did not move, because the
	// array shape encodes byte for byte what the wire profile did — which is
	// what shows the encoding changed and the simulation did not.
	const wantTick, wantWorld, wantAction = 101, "959a52fe141be21f", "d8cd69f005b7cc42"
	if last.Tick != wantTick || last.WorldHash != wantWorld || last.ActionHash != wantAction {
		t.Fatalf("final checkpoint tick %d (of %d) world %s action %s (pinned %d / %s / %s)",
			last.Tick, ticks, last.WorldHash, last.ActionHash, wantTick, wantWorld, wantAction)
	}
}
