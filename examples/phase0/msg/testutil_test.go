package msg_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// moduleDir resolves the on-disk directory of a dependency of the module
// under test (its module cache entry, or its replacement directory), so the
// fixture module builds against exactly the code this package builds
// against. The test runs with the package directory as its working
// directory, which is inside the module.
func moduleDir(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", path)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list -m %s: %v\n%s", path, err, errOut.String())
	}
	dir := strings.TrimSpace(out.String())
	if dir == "" {
		t.Fatalf("no directory resolved for %s", path)
	}
	return dir
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out.String())
	}
}
