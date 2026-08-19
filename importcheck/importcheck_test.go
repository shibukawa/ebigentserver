package importcheck

import (
	"context"
	"strings"
	"testing"
)

func TestCheckReportsConfinedImports(t *testing.T) {
	violations, err := Check(context.Background(), "testdata/game", Default())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	got := make(map[string]Violation, len(violations))
	for _, v := range violations {
		got[v.Pkg] = v
		t.Logf("violation: %s", v)
	}

	// The rules package imports the engine directly.
	rules, ok := got["example.com/game/rules"]
	if !ok {
		t.Fatalf("expected a violation for example.com/game/rules, got %v", violations)
	}
	if rules.Target != "github.com/hajimehoshi/ebiten/v2" {
		t.Errorf("rules violation target = %q", rules.Target)
	}
	wantChain := "example.com/game/rules -> github.com/hajimehoshi/ebiten/v2"
	if strings.Join(rules.Chain, " -> ") != wantChain {
		t.Errorf("rules chain = %v, want %s", rules.Chain, wantChain)
	}

	// The sim entry point acquires the engine transitively through rules;
	// the chain must name the package that introduced it.
	sim, ok := got["example.com/game/cmd/game-sim"]
	if !ok {
		t.Fatalf("expected a violation for cmd/game-sim, got %v", violations)
	}
	wantChain = "example.com/game/cmd/game-sim -> example.com/game/rules -> github.com/hajimehoshi/ebiten/v2"
	if strings.Join(sim.Chain, " -> ") != wantChain {
		t.Errorf("sim chain = %v, want %s", sim.Chain, wantChain)
	}

	// Client entry point and presentation package are allowed; session and
	// the dedicated server are clean. Nothing else may be reported.
	for pkg := range got {
		switch pkg {
		case "example.com/game/rules", "example.com/game/cmd/game-sim":
		default:
			t.Errorf("unexpected violation for %s: %s", pkg, got[pkg])
		}
	}
}

func TestEnforceFailsOnViolation(t *testing.T) {
	rec := &recorder{}
	Enforce(rec, "testdata/game", Default())
	if rec.errors == 0 {
		t.Fatal("Enforce reported no violations for the fixture module")
	}
	if rec.fatals != 0 {
		t.Fatalf("Enforce aborted: %v", rec.messages)
	}
}

type recorder struct {
	errors   int
	fatals   int
	messages []string
}

func (r *recorder) Helper() {}
func (r *recorder) Fatalf(format string, args ...any) {
	r.fatals++
	r.messages = append(r.messages, format)
}
func (r *recorder) Errorf(format string, args ...any) {
	r.errors++
	r.messages = append(r.messages, format)
}
