package admission

import (
	"errors"
	"fmt"
	"net"
)

// ErrPublicUnauthenticated fails startup of an unauthenticated session
// on a publicly reachable listener.
var ErrPublicUnauthenticated = errors.New("admission: unauthenticated session must not listen on a public address")

// GuardUnauthenticated enforces the network_scope arm of
// rule:unauthenticated-admission-requires-scope-or-capability: a
// session that admits agents without a data:session-ticket may only
// listen where network scope itself is the control — loopback, RFC1918
// private ranges, or link-local (concept:listen-server-mode on a LAN,
// api:lan-discovery). Anything else — including the unspecified
// addresses 0.0.0.0 and ::, which expose every interface — fails
// closed, as do unresolvable hosts.
//
// The other valid control, rendezvous capability, needs no guard here:
// a WebRTC session admitted via an api:manual-signaling-token has no
// listening port at all, so there is nothing to scan or enumerate —
// its check is rule:invitation-is-single-use-and-expiring instead.
//
// Call it at session startup and again at flow:session-admission, not
// only at configuration load. Overriding it is an explicit operator
// opt-in, never the default (decision:no-auth-on-lan).
func GuardUnauthenticated(listenAddr string) error {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		host = listenAddr // tolerate a bare host with no port
	}
	if host == "" {
		// ":8080" style — listens on every interface.
		return fmt.Errorf("%w: %q binds all interfaces", ErrPublicUnauthenticated, listenAddr)
	}
	ips := []net.IP{}
	if ip := net.ParseIP(host); ip != nil {
		ips = append(ips, ip)
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil || len(resolved) == 0 {
			return fmt.Errorf("%w: cannot resolve %q", ErrPublicUnauthenticated, host)
		}
		ips = resolved
	}
	// Every address the name maps to must be scoped; one public A
	// record fails the whole guard.
	for _, ip := range ips {
		if ip.IsUnspecified() {
			return fmt.Errorf("%w: %v binds all interfaces", ErrPublicUnauthenticated, ip)
		}
		if !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() {
			return fmt.Errorf("%w: %v", ErrPublicUnauthenticated, ip)
		}
	}
	return nil
}
