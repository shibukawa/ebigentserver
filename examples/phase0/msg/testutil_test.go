package msg_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// moduleVersion resolves the version of a dependency of the module under
// test, so the fixture module pins the same versions this package builds
// against. The test runs with the package directory as its working
// directory, which is inside the module.
func moduleVersion(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}{{if .Replace}} replaced{{end}}", path)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list -m %s: %v\n%s", path, err, errOut.String())
	}
	version := strings.TrimSpace(out.String())
	if strings.HasSuffix(version, "replaced") {
		t.Skipf("%s is replaced locally; fixture module cannot pin it", path)
	}
	if version == "" {
		t.Fatalf("no version resolved for %s", path)
	}
	return version
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
