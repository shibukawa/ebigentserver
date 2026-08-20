//go:build windows

package discovery

import "syscall"

// broadcastControl enables SO_BROADCAST on the announcer's socket —
// required to send to 255.255.255.255; harmless for the unicast
// loopback destinations tests use.
func broadcastControl(network, address string, c syscall.RawConn) error {
	var serr error
	err := c.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
