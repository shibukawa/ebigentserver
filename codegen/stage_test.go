package codegen_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/ebigentserver/codegen"
)

// RuleSets exists so a tool can learn a game's types without linking the
// game. All three positions matter here where Stages needs two: an agent
// is written against the sight and the action, and either may live in a
// package the agent's own file does not import yet.
func TestRuleSetsResolvesAllThreePositions(t *testing.T) {
	root := t.TempDir()
	writeUnder(t, root, "go.mod", "module example.com/game\n\ngo 1.25\n")
	writeUnder(t, root, "msg/types.go", "package msg\n\ntype World struct{}\ntype Move struct{}\n")
	writeUnder(t, root, "rules/rules.go", `package rules

import (
	"github.com/shibukawa/ebigentserver/session"
	"example.com/game/msg"
)

type Sight struct{}
type RuleSet struct{}

var _ session.TickStageRuleSet[msg.World, msg.Move, Sight] = RuleSet{}
`)

	sets, err := codegen.RuleSets(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 {
		t.Fatalf("found %d rule sets, want 1", len(sets))
	}
	rs := sets[0]
	if rs.Package != "rules" {
		t.Errorf("package = %q, want rules", rs.Package)
	}
	// Qualified is the point: the same type is spelled two ways
	// depending on which package is doing the writing.
	if got := rs.Sight.Qualified(rs.Dir); got != "Sight" {
		t.Errorf("the sight is declared here, so it is spelled bare; got %q", got)
	}
	if got := rs.Action.Qualified(rs.Dir); got != "msg.Move" {
		t.Errorf("the action is elsewhere, so it needs a qualifier; got %q", got)
	}
	if rs.Action.Import != "example.com/game/msg" {
		t.Errorf("action import = %q", rs.Action.Import)
	}
	if rs.World.Name != "World" || rs.World.Import != "example.com/game/msg" {
		t.Errorf("world = %+v", rs.World)
	}
}

// A repository holding several games reports several, in a stable order,
// so a caller can name one rather than be handed a guess.
func TestRuleSetsReportsEveryGame(t *testing.T) {
	root := t.TempDir()
	writeUnder(t, root, "go.mod", "module example.com/games\n\ngo 1.25\n")
	for _, name := range []string{"beta", "alpha"} {
		writeUnder(t, root, name+"/"+name+".go", "package "+name+`

import "github.com/shibukawa/ebigentserver/session"

type World struct{}
type Action struct{}
type Sight struct{}
type RuleSet struct{}

var _ session.StageRuleSet[World, Action, Sight] = RuleSet{}
`)
	}
	sets, err := codegen.RuleSets(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range sets {
		names = append(names, s.Package)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("packages = %v, want [alpha beta] in that order", names)
	}
}

// writeUnder writes a file at a path with directories in it, which the
// shared write helper does not create.
func writeUnder(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
