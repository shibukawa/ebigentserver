package admission

import (
	"errors"
	"testing"
)

// rule:unauthenticated-admission-requires-scope-or-capability: fail
// closed unless the listener is loopback, private, or link-local.
func TestGuardUnauthenticated(t *testing.T) {
	cases := []struct {
		addr string
		ok   bool
	}{
		{"127.0.0.1:8080", true},
		{"localhost:8080", true},
		{"[::1]:8080", true},
		{"192.168.1.20:9000", true},
		{"10.0.0.7:4433", true},
		{"172.16.5.5:1", true},
		{"169.254.10.10:5", true}, // link-local
		{"[fe80::1]:8080", true},  // link-local v6
		{"[fd00::1]:8080", true},  // ULA counts as private
		{"0.0.0.0:8080", false},   // all interfaces
		{"[::]:8080", false},
		{":8080", false}, // empty host = all interfaces
		{"203.0.113.9:443", false},
		{"8.8.8.8:53", false},
		{"[2001:db8::1]:443", false},
		{"definitely-not-resolvable.invalid:1", false}, // fail closed
	}
	for _, c := range cases {
		err := GuardUnauthenticated(c.addr)
		if c.ok && err != nil {
			t.Errorf("GuardUnauthenticated(%q) = %v, want nil", c.addr, err)
		}
		if !c.ok {
			if err == nil {
				t.Errorf("GuardUnauthenticated(%q) = nil, want rejection", c.addr)
			} else if !errors.Is(err, ErrPublicUnauthenticated) {
				t.Errorf("GuardUnauthenticated(%q) = %v, want ErrPublicUnauthenticated", c.addr, err)
			}
		}
	}
}
