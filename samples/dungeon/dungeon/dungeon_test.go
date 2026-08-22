package dungeon_test

import (
	"testing"

	"github.com/shibukawa/ebigentserver/samples/dungeon/dungeon"
	"github.com/shibukawa/ebigentserver/samples/dungeon/msg"
	"github.com/shibukawa/ebigentserver/session"
)

func newRuleSet(adventurers int) (dungeon.RuleSet, dungeon.State) {
	g := dungeon.RuleSet{Adventurers: adventurers, TickLimit: 600}
	return g, g.Start(42)
}

func TestMazeIsDeterministic(t *testing.T) {
	g := dungeon.RuleSet{Adventurers: 4}
	a, b := g.Start(7), g.Start(7)
	for i := range a.Walls {
		if a.Walls[i] != b.Walls[i] {
			t.Fatal("same seed produced different mazes")
		}
	}
	c := g.Start(8)
	same := true
	for i := range a.Walls {
		if a.Walls[i] != c.Walls[i] {
			same = false
		}
	}
	if same {
		t.Fatal("different seeds produced identical mazes")
	}
}

func TestTrapSpringsAndDiscovers(t *testing.T) {
	g, s := newRuleSet(1) // scout only
	v := dungeon.Validator{}
	scout := session.SlotID(2)
	// Clear the corridor cells the test walks, whatever the seed rolled.
	for _, x := range []uint8{3, 4} {
		i := int(2)*msg.GridW + int(x)
		s.Walls[i/8] &^= 1 << (i % 8)
	}

	// The DM plants a trap right of the scout's spawn corridor.
	place := dungeon.Input{Kind: msg.ActPlaceTrap, X: 3, Y: 2}
	if err := v.Legal(&s, dungeon.SlotDM, place); err != nil {
		t.Fatalf("legal trap rejected: %v", err)
	}
	g.Apply(&s, dungeon.SlotDM, place)
	if s.TrapBudget != 7 || len(s.Traps) != 1 {
		t.Fatalf("trap not booked: %+v", s)
	}
	// The scout walks onto it: damage, disarm, discovery.
	g.Apply(&s, scout, dungeon.Input{Kind: msg.ActMove, Dir: 1})
	if s.Adventurers[0].HP != 1 {
		t.Fatalf("HP = %d, want 1", s.Adventurers[0].HP)
	}
	if s.Traps[0].Armed || !s.Traps[0].Discovered {
		t.Fatalf("sprung trap state: %+v", s.Traps[0])
	}
	// A second trap and a second spring kills; the DM wins.
	g.Apply(&s, dungeon.SlotDM, dungeon.Input{Kind: msg.ActPlaceTrap, X: 4, Y: 2})
	g.Apply(&s, scout, dungeon.Input{Kind: msg.ActMove, Dir: 1})
	if s.Adventurers[0].Alive {
		t.Fatal("scout survived a second trap at 1 HP")
	}
	g.Advance(&s)
	if !s.Over || s.Winner != 2 {
		t.Fatalf("DM should win when the party is dead: %+v", s)
	}
}

func TestRoleGating(t *testing.T) {
	g, s := newRuleSet(4)
	v := dungeon.Validator{}
	scout, engineer := session.SlotID(2), session.SlotID(3)

	// Adventurers cannot place traps; the DM cannot move.
	if err := v.Legal(&s, scout, dungeon.Input{Kind: msg.ActPlaceTrap, X: 5, Y: 5}); err == nil {
		t.Error("adventurer placed a trap")
	}
	if err := v.Legal(&s, dungeon.SlotDM, dungeon.Input{Kind: msg.ActMove, Dir: 1}); err == nil {
		t.Error("DM moved")
	}
	// Only the engineer disarms, only adjacent discovered traps.
	g.Apply(&s, dungeon.SlotDM, dungeon.Input{Kind: msg.ActPlaceTrap, X: 4, Y: 3})
	s.Traps[0].Discovered = true
	if err := v.Legal(&s, scout, dungeon.Input{Kind: msg.ActDisarm, X: 4, Y: 3}); err == nil {
		t.Error("scout disarmed")
	}
	if err := v.Legal(&s, engineer, dungeon.Input{Kind: msg.ActDisarm, X: 4, Y: 3}); err != nil {
		t.Errorf("adjacent engineer disarm rejected: %v", err)
	}
	g.Apply(&s, engineer, dungeon.Input{Kind: msg.ActDisarm, X: 4, Y: 3})
	if s.Traps[0].Armed {
		t.Error("disarm did not disarm")
	}
}

// The projection invariants: what a view must never contain
// (concept:visibility-scope's security note — hidden data is never sent,
// and these views are the only thing that is ever serialized).
func assertAdventurerViewInvariants(t *testing.T, v *msg.AdventurerView, navigator bool) {
	t.Helper()
	for _, tr := range v.KnownTraps {
		if !tr.Discovered {
			t.Fatalf("undiscovered trap %d leaked into an adventurer view", tr.ID)
		}
	}
	for i := range v.KnownWalls {
		if v.KnownWalls[i]&^v.Explored[i] != 0 {
			t.Fatal("wall knowledge outside explored cells leaked")
		}
	}
	if !navigator && (v.ExitX != msg.Unknown || v.ExitY != msg.Unknown) {
		t.Fatal("exit position leaked to a non-navigator")
	}
}

func TestProjectionsHideAndReveal(t *testing.T) {
	g, s := newRuleSet(4)
	scout, navigator := session.SlotID(2), session.SlotID(5)

	// A trap far from the party is invisible to every adventurer but
	// fully visible to the DM.
	g.Apply(&s, dungeon.SlotDM, dungeon.Input{Kind: msg.ActPlaceTrap, X: msg.GridW - 4, Y: msg.GridH - 4})
	for _, slot := range []session.SlotID{2, 3, 4, 5} {
		v := dungeon.ProjectAdventurer(&s, slot)
		assertAdventurerViewInvariants(t, &v, slot == navigator)
		if len(v.KnownTraps) != 0 {
			t.Fatalf("slot %d sees the hidden trap", slot)
		}
	}
	dm := dungeon.ProjectDM(&s)
	if len(dm.Traps) != 1 || dm.Traps[0].Discovered {
		t.Fatalf("DM view traps: %+v", dm.Traps)
	}
	// The navigator alone knows the exit; the treasure is unknown until
	// its cell is explored.
	sv := dungeon.ProjectAdventurer(&s, scout)
	nv := dungeon.ProjectAdventurer(&s, navigator)
	if nv.ExitX != s.ExitX || nv.ExitY != s.ExitY {
		t.Fatal("navigator does not know the exit")
	}
	if sv.TreasureX != msg.Unknown {
		t.Fatal("unexplored treasure position leaked")
	}
	// Explore the treasure cell: it appears for the whole team.
	a := &s.Adventurers[0]
	a.X, a.Y = s.TreasureX, s.TreasureY-1
	g.Advance(&s)
	sv = dungeon.ProjectAdventurer(&s, scout)
	if sv.TreasureX != s.TreasureX {
		t.Fatal("explored treasure still hidden")
	}
	// Discovery by adjacency reveals a trap to the team.
	g.Apply(&s, dungeon.SlotDM, dungeon.Input{Kind: msg.ActPlaceTrap, X: a.X + 1, Y: a.Y})
	g.Advance(&s)
	sv = dungeon.ProjectAdventurer(&s, scout)
	found := false
	for _, tr := range sv.KnownTraps {
		if tr.X == a.X+1 && tr.Y == a.Y {
			found = true
		}
	}
	if !found {
		t.Fatal("adjacent trap not discovered")
	}
	assertAdventurerViewInvariants(t, &sv, false)
}

// rule:evaluation-respects-visibility-scope — a hidden DM action must not
// move the party's numbers.
func TestEvaluationIsScoped(t *testing.T) {
	g, s := newRuleSet(2)
	scout := session.SlotID(2)
	before := g.Evaluate(&s, scout)
	g.Apply(&s, dungeon.SlotDM, dungeon.Input{Kind: msg.ActPlaceTrap, X: msg.GridW - 4, Y: msg.GridH - 4})
	after := g.Evaluate(&s, scout)
	if before != after {
		t.Fatalf("hidden trap moved the scoped signal: %+v -> %+v", before, after)
	}
	// The DM's own (privileged, annotated as such) signal may of course
	// see everything; the annotation says so.
	ann := dungeon.DMAnnotation(nil)
	if ann.EvaluationScope != "privileged" {
		t.Fatalf("DM annotation: %+v", ann)
	}
	v := dungeon.ProjectAdventurer(&s, scout)
	if a := dungeon.AdventurerAnnotation(&v); a.EvaluationScope != "scoped" || a.Scope != "team" {
		t.Fatalf("adventurer annotation: %+v", a)
	}
}

// The party wins: the test plans a route with full knowledge (tests may
// cheat; players may not) and drives the carrier through it.
func TestPartyCanWin(t *testing.T) {
	g, s := newRuleSet(4)
	carrier := session.SlotID(4)
	v := dungeon.Validator{}

	route := planRoute(t, &s, s.Adventurers[2].X, s.Adventurers[2].Y, s.TreasureX, s.TreasureY)
	route = append(route, planRoute(t, &s, s.TreasureX, s.TreasureY, s.ExitX, s.ExitY)...)
	for _, dir := range route {
		in := dungeon.Input{Kind: msg.ActMove, Dir: dir}
		if err := v.Legal(&s, carrier, in); err != nil {
			t.Fatal(err)
		}
		g.Apply(&s, carrier, in)
		g.Advance(&s)
		if s.Over {
			break
		}
	}
	if !s.Over || s.Winner != 1 {
		t.Fatalf("party did not win: over=%v winner=%d", s.Over, s.Winner)
	}
	sig := g.Evaluate(&s, carrier)
	if sig.Terminal != session.Win {
		t.Fatalf("carrier terminal = %v", sig.Terminal)
	}
	if g.Evaluate(&s, dungeon.SlotDM).Terminal != session.Lose {
		t.Fatal("DM terminal not Lose")
	}
}

// planRoute BFSes over the full maze — test-side omniscience.
func planRoute(t *testing.T, s *dungeon.State, fx, fy, tx, ty uint8) []uint8 {
	t.Helper()
	type node struct{ x, y uint8 }
	prev := map[node]node{}
	dirTo := map[node]uint8{}
	start := node{fx, fy}
	queue := []node{start}
	seen := map[node]bool{start: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.x == tx && cur.y == ty {
			var dirs []uint8
			for cur != start {
				dirs = append([]uint8{dirTo[cur]}, dirs...)
				cur = prev[cur]
			}
			return dirs
		}
		for dir := uint8(0); dir < 4; dir++ {
			nx, ny, ok := stepTest(cur.x, cur.y, dir)
			if !ok || dungeon.Bit(s.Walls, nx, ny) || seen[node{nx, ny}] {
				continue
			}
			seen[node{nx, ny}] = true
			prev[node{nx, ny}] = cur
			dirTo[node{nx, ny}] = dir
			queue = append(queue, node{nx, ny})
		}
	}
	t.Fatalf("no route from (%d,%d) to (%d,%d)", fx, fy, tx, ty)
	return nil
}

func stepTest(x, y, dir uint8) (uint8, uint8, bool) {
	switch dir {
	case 0:
		if y == 0 {
			return 0, 0, false
		}
		return x, y - 1, true
	case 1:
		if x >= msg.GridW-1 {
			return 0, 0, false
		}
		return x + 1, y, true
	case 2:
		if y >= msg.GridH-1 {
			return 0, 0, false
		}
		return x, y + 1, true
	default:
		if x == 0 {
			return 0, 0, false
		}
		return x - 1, y, true
	}
}
