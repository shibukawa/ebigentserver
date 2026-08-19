package importcheck

import "context"

// TestingT is the subset of *testing.T the Enforce helper needs.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Enforce runs Check over the module rooted at dir and reports every
// violation as a test failure. A game module keeps the rule enforced by
// carrying one test:
//
//	func TestImportBoundary(t *testing.T) {
//		importcheck.Enforce(t, ".", importcheck.Default())
//	}
func Enforce(t TestingT, dir string, cfg Config) {
	t.Helper()
	violations, err := Check(context.Background(), dir, cfg)
	if err != nil {
		t.Fatalf("importcheck: %v", err)
	}
	for _, v := range violations {
		t.Errorf("%s", v.String())
	}
}
