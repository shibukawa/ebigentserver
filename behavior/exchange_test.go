package behavior

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func exchangeFixture() (AnalysisRequest, *Proposals) {
	req := AnalysisRequest{
		Game: "test",
		Features: []RequestFeature{
			{Name: "f0", GoExpr: "e0"}, {Name: "f1", GoExpr: "e1"}, {Name: "f2", GoExpr: "e2"},
		},
		Actions: []RequestAction{
			{Name: "a0", GoExpr: "g0"}, {Name: "a1", GoExpr: "g1"},
		},
		Records: []RequestRecord{
			{Episode: "e-1", Tick: 1, Slot: 1, Action: "a0", Bits: "100"},
			{Episode: "e-1", Tick: 2, Slot: 1, Action: "a0", Bits: "110"},
			{Episode: "e-2", Tick: 1, Slot: 1, Action: "a1", Bits: "011"},
			{Episode: "e-2", Tick: 2, Slot: 1, Action: "a1", Bits: "010"},
		},
	}
	props := &Proposals{
		Game: "test",
		Candidates: []Candidate{
			// Claimed numbers are wrong on purpose; the validator recomputes.
			{Condition: "f0", Action: "a0", Coverage: 999, Counterexamples: 3,
				Evidence: []Evidence{{Episode: "e-1", Tick: 1}, {Episode: "ghost", Tick: 9}}},
			// A dropped rule (wrong action) must not eat f1's records...
			{Condition: "f1", Action: "a0"},
			// ...so this one still validates.
			{Condition: "f1", Action: "a1"},
			// Hallucinated predicate: rejected outright.
			{Condition: "not_in_vocabulary", Action: "a1"},
		},
	}
	return req, props
}

func TestValidateProposalsRecomputesAndRejects(t *testing.T) {
	req, props := exchangeFixture()
	valid, issues := ValidateProposals(req, props)

	if len(valid) != 2 {
		t.Fatalf("valid = %+v, want 2", valid)
	}
	// f0→a0: records 1 and 2 (bits[0]=='1'), both a0.
	if valid[0].Coverage != 2 || valid[0].Counterexamples != 0 {
		t.Fatalf("f0→a0 recomputed as %d/%d", valid[0].Coverage, valid[0].Counterexamples)
	}
	// f1→a1 after the dropped f1→a0 and after f0→a0 consumed record 2:
	// remaining f1 matches are records 3 and 4, both a1. The tentative-
	// matching rule is what keeps this at 2 instead of 0.
	if valid[1].Condition != "f1" || valid[1].Action != "a1" || valid[1].Coverage != 2 {
		t.Fatalf("f1→a1 = %+v", valid[1])
	}
	kinds := map[string]int{}
	for _, is := range issues {
		kinds[is.Kind]++
	}
	for _, want := range []string{"coverage_corrected", "evidence_invalid", "no_coverage", "unknown_condition"} {
		if kinds[want] == 0 {
			t.Fatalf("missing issue kind %s in %+v", want, issues)
		}
	}
}

// The Python validator that ships with the skill must agree with the Go
// one bit for bit — it is the same trust boundary running in the
// analyst's environment.
func TestPythonValidatorParity(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	req, props := exchangeFixture()
	dir := t.TempDir()
	reqPath := filepath.Join(dir, "request.json")
	if err := req.Save(reqPath); err != nil {
		t.Fatal(err)
	}
	propsPath := filepath.Join(dir, "proposals.json")
	pb, _ := json.Marshal(props)
	if err := os.WriteFile(propsPath, pb, 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "validated.json")
	script := filepath.Join("..", "skills", "behavior-analyze", "scripts", "validate_proposals.py")
	cmd := exec.Command("python3", script, "--request", reqPath, "--proposals", propsPath, "--out", outPath)
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); err != nil && (!ok || ee.ExitCode() != 1) {
		// exit 1 = validated with issues, which this fixture has.
		t.Fatalf("python validator: %v\n%s", err, out)
	}
	validated, err := LoadProposals(outPath)
	if err != nil {
		t.Fatal(err)
	}
	goValid, _ := ValidateProposals(req, props)
	if len(validated.Candidates) != len(goValid) {
		t.Fatalf("python kept %d candidates, Go kept %d\n%s", len(validated.Candidates), len(goValid), out)
	}
	for i := range goValid {
		p, g := validated.Candidates[i], goValid[i]
		if p.Condition != g.Condition || p.Action != g.Action ||
			p.Coverage != g.Coverage || p.Counterexamples != g.Counterexamples {
			t.Fatalf("candidate %d diverged: python %+v vs go %+v", i, p, g)
		}
	}
}

func TestRequestRoundTrip(t *testing.T) {
	req, _ := exchangeFixture()
	dir := t.TempDir()
	path := filepath.Join(dir, "req.json")
	if err := req.Save(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back AnalysisRequest
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Records) != 4 || back.Records[1].Bits != "110" || back.Features[2].Name != "f2" {
		t.Fatalf("round trip mangled the request: %+v", back)
	}
}
